package agentapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultBaseURL       = "https://api.agentsway.dev"
	DefaultTimeout       = 10 * time.Minute
	DefaultStreamTimeout = time.Hour
	DefaultMaxRetries    = 2
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type ClientOptions struct {
	APIKey         string
	BaseURL        string
	HTTPClient     HTTPDoer
	Timeout        time.Duration
	StreamTimeout  time.Duration
	MaxRetries     int
	DefaultHeaders map[string]string
}

type httpClient struct {
	baseURL        string
	apiKey         string
	client         HTTPDoer
	timeout        time.Duration
	streamTimeout  time.Duration
	maxRetries     int
	defaultHeaders map[string]string
}

func newHTTPClient(opts *ClientOptions) *httpClient {
	if opts == nil {
		opts = &ClientOptions{}
	}
	baseURL := strings.TrimRight(firstNonEmpty(opts.BaseURL, os.Getenv("AGENT_API_BASE_URL"), DefaultBaseURL), "/")
	apiKey := firstNonEmpty(opts.APIKey, os.Getenv("AGENT_API_KEY"))
	hc := opts.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	streamTimeout := opts.StreamTimeout
	if streamTimeout <= 0 {
		streamTimeout = DefaultStreamTimeout
	}
	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	} else if maxRetries == 0 {
		maxRetries = DefaultMaxRetries
	}
	headers := map[string]string{}
	for k, v := range opts.DefaultHeaders {
		headers[k] = v
	}
	return &httpClient{
		baseURL:        baseURL,
		apiKey:         apiKey,
		client:         hc,
		timeout:        timeout,
		streamTimeout:  streamTimeout,
		maxRetries:     maxRetries,
		defaultHeaders: headers,
	}
}

func (c *httpClient) requestJSON(ctx context.Context, method, path string, body any, out any, opts ...RequestOption) error {
	resp, err := c.do(ctx, method, path, body, false, opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *httpClient) requestRawJSON(ctx context.Context, method, path string, body io.Reader, out any, opts ...RequestOption) error {
	resp, err := c.doRaw(ctx, method, path, body, false, "application/octet-stream", opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *httpClient) requestBinary(ctx context.Context, method, path string, opts ...RequestOption) ([]byte, http.Header, error) {
	resp, err := c.do(ctx, method, path, nil, false, opts...)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header.Clone(), err
}

func (c *httpClient) stream(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, method, path, body, true, opts...)
}

func (c *httpClient) do(ctx context.Context, method, path string, body any, stream bool, opts ...RequestOption) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	return c.doRaw(ctx, method, path, reader, stream, "application/json", opts...)
}

func (c *httpClient) doRaw(ctx context.Context, method, path string, body io.Reader, stream bool, contentType string, opts ...RequestOption) (*http.Response, error) {
	ro := requestOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&ro)
		}
	}
	timeout := ro.timeout
	if timeout <= 0 {
		if stream {
			timeout = c.streamTimeout
		} else {
			timeout = c.timeout
		}
	}
	maxRetries := c.maxRetries
	if ro.maxRetries != nil {
		maxRetries = *ro.maxRetries
	}
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; ; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := c.doOnce(reqCtx, method, path, bodyBytes, stream, contentType, ro.headers)
		if err == nil {
			if resp.Body != nil {
				resp.Body = cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
			} else {
				cancel()
			}
			return resp, nil
		}
		cancel()
		if attempt >= maxRetries || !retryableError(err) {
			return nil, err
		}
		sleep := retryDelay(err, attempt+1)
		select {
		case <-ctx.Done():
			return nil, &APIConnectionError{Message: "request cancelled", Err: ctx.Err()}
		case <-time.After(sleep):
		}
	}
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func (c *httpClient) doOnce(ctx context.Context, method, path string, body []byte, stream bool, contentType string, headers map[string]string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("User-Agent", userAgent)
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &APIConnectionError{Message: "request failed", Err: err}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return nil, parseResponseError(resp, raw)
}

func retryableError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*APIConnectionError); ok {
		return true
	}
	if r, ok := err.(*RateLimitError); ok {
		return r.Status == http.StatusTooManyRequests
	}
	if apiErr, ok := err.(*APIError); ok {
		return isRetryableStatus(apiErr.Status)
	}
	return false
}

func retryDelay(err error, attempt int) time.Duration {
	if r, ok := err.(*RateLimitError); ok && r.RetryAfter > 0 {
		return r.RetryAfter
	}
	base := 250 * time.Millisecond
	capDelay := 8 * time.Second
	delay := base << max(0, attempt-1)
	if delay > capDelay {
		delay = capDelay
	}
	return delay + time.Duration(rand.Int64N(int64(100*time.Millisecond)))
}

type requestOptions struct {
	timeout    time.Duration
	maxRetries *int
	headers    map[string]string
}

type RequestOption func(*requestOptions)

func WithTimeout(timeout time.Duration) RequestOption {
	return func(o *requestOptions) { o.timeout = timeout }
}

func WithMaxRetries(maxRetries int) RequestOption {
	return func(o *requestOptions) { o.maxRetries = &maxRetries }
}

func WithHeader(key, value string) RequestOption {
	return func(o *requestOptions) {
		if o.headers == nil {
			o.headers = map[string]string{}
		}
		o.headers[key] = value
	}
}

func buildQuery(values map[string]any) string {
	q := url.Values{}
	for key, value := range values {
		switch v := value.(type) {
		case nil:
		case string:
			if v != "" {
				q.Set(key, v)
			}
		case bool:
			q.Set(key, fmt.Sprintf("%t", v))
		case int:
			if v != 0 {
				q.Set(key, fmt.Sprintf("%d", v))
			}
		case int64:
			if v != 0 {
				q.Set(key, fmt.Sprintf("%d", v))
			}
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func pathEscapePath(path string) string {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, url.PathEscape(part))
	}
	return strings.Join(out, "/")
}

func normalizeArchivePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")
	return strings.ReplaceAll(path, "//", "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
