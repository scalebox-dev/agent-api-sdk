package agentapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIError struct {
	Message   string
	Status    int
	Header    http.Header
	Body      []byte
	Code      string
	Type      string
	RequestID string
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Status > 0 {
		return fmt.Sprintf("agentapi: status %d: %s", e.Status, e.Message)
	}
	return "agentapi: " + e.Message
}

type APIConnectionError struct {
	Message string
	Err     error
}

func (e *APIConnectionError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return "agentapi: " + e.Message + ": " + e.Err.Error()
	}
	return "agentapi: " + e.Message
}

func (e *APIConnectionError) Unwrap() error { return e.Err }

type RateLimitError struct {
	*APIError
	RetryAfter time.Duration
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func parseResponseError(resp *http.Response, body []byte) error {
	apiErr := &APIError{
		Message:   fmt.Sprintf("API request failed with status %d", resp.StatusCode),
		Status:    resp.StatusCode,
		Header:    resp.Header.Clone(),
		Body:      append([]byte(nil), body...),
		RequestID: firstHeader(resp.Header, "x-request-id", "request-id"),
	}
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil {
		if strings.TrimSpace(parsed.Error.Message) != "" {
			apiErr.Message = parsed.Error.Message
		}
		apiErr.Code = parsed.Error.Code
		apiErr.Type = parsed.Error.Type
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{APIError: apiErr, RetryAfter: retryAfter(resp.Header)}
	}
	return apiErr
}

func retryAfter(h http.Header) time.Duration {
	raw := strings.TrimSpace(h.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil {
		return time.Duration(seconds * float64(time.Second))
	}
	if t, err := http.ParseTime(raw); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func firstHeader(h http.Header, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(h.Get(name)); v != "" {
			return v
		}
	}
	return ""
}
