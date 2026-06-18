package agentapi

import (
	"context"
	"errors"
	"net/url"
	"time"
)

const defaultDevicePollInterval = 5 * time.Second

type AuthService struct {
	http *httpClient
}

type StartDeviceAuthParams struct {
	ClientName string `json:"client_name,omitempty"`
}

type DeviceAuthStart struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresAt               int64  `json:"expires_at"`
	IntervalSeconds         int    `json:"interval_seconds"`
}

type PollDeviceAuthParams struct {
	DeviceCode string `json:"device_code"`
}

type RefreshBrowserSessionParams struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthSession struct {
	AccessToken           string   `json:"access_token"`
	RefreshToken          string   `json:"refresh_token"`
	TokenType             string   `json:"token_type,omitempty"`
	AccessTokenExpiresAt  int64    `json:"access_token_expires_at"`
	RefreshTokenExpiresAt int64    `json:"refresh_token_expires_at,omitempty"`
	UserID                string   `json:"user_id"`
	WorkspaceID           string   `json:"workspace_id"`
	WorkspaceName         string   `json:"workspace_name,omitempty"`
	WorkspaceRole         string   `json:"workspace_role"`
	Scopes                []string `json:"scopes"`
}

type DeviceAuthPollResult struct {
	Status           string   `json:"status"`
	Message          string   `json:"message,omitempty"`
	IntervalSeconds  int      `json:"interval_seconds,omitempty"`
	ExpiresAt        int64    `json:"expires_at,omitempty"`
	AccessToken      string   `json:"access_token,omitempty"`
	RefreshToken     string   `json:"refresh_token,omitempty"`
	TokenType        string   `json:"token_type,omitempty"`
	AccessExpiresAt  int64    `json:"access_token_expires_at,omitempty"`
	RefreshExpiresAt int64    `json:"refresh_token_expires_at,omitempty"`
	UserID           string   `json:"user_id,omitempty"`
	WorkspaceID      string   `json:"workspace_id,omitempty"`
	WorkspaceName    string   `json:"workspace_name,omitempty"`
	WorkspaceRole    string   `json:"workspace_role,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
}

type WaitForDeviceAuthOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	OnPoll   func(*DeviceAuthPollResult)
}

type DeviceAuthFlowError struct {
	Message string
	Result  *DeviceAuthPollResult
}

func (e *DeviceAuthFlowError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "device auth flow failed"
}

func (s *AuthService) StartDeviceAuth(ctx context.Context, params StartDeviceAuthParams, opts ...RequestOption) (*DeviceAuthStart, error) {
	var out DeviceAuthStart
	if err := s.http.requestJSON(ctx, "POST", "/v1/auth/device/start", params, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *AuthService) PollDeviceAuth(ctx context.Context, params PollDeviceAuthParams, opts ...RequestOption) (*DeviceAuthPollResult, error) {
	if params.DeviceCode == "" {
		return nil, errors.New("device_code is required")
	}
	var out DeviceAuthPollResult
	if err := s.http.requestJSON(ctx, "POST", "/v1/auth/device/poll", params, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *AuthService) RefreshBrowserSession(ctx context.Context, params RefreshBrowserSessionParams, opts ...RequestOption) (*AuthSession, error) {
	if params.RefreshToken == "" {
		return nil, errors.New("refresh_token is required")
	}
	opts = append([]RequestOption{
		WithHeader("Cookie", "agent_api_refresh="+url.PathEscape(params.RefreshToken)),
	}, opts...)
	var out AuthSession
	if err := s.http.requestJSON(ctx, "POST", "/v1/auth/refresh", map[string]any{}, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *AuthService) WaitForDeviceAuth(ctx context.Context, params PollDeviceAuthParams, wait WaitForDeviceAuthOptions, opts ...RequestOption) (*AuthSession, error) {
	if params.DeviceCode == "" {
		return nil, errors.New("device_code is required")
	}
	started := time.Now()
	for {
		result, err := s.PollDeviceAuth(ctx, params, opts...)
		if err != nil {
			return nil, err
		}
		if wait.OnPoll != nil {
			wait.OnPoll(result)
		}
		if result.Status == "approved" && result.AccessToken != "" && result.RefreshToken != "" {
			return result.AuthSession(), nil
		}
		if result.Status == "expired" || result.Status == "consumed" {
			return nil, &DeviceAuthFlowError{Message: firstNonEmpty(result.Message, "device auth "+result.Status), Result: result}
		}
		if wait.Timeout > 0 && time.Since(started) >= wait.Timeout {
			return nil, &DeviceAuthFlowError{Message: "device auth timed out", Result: result}
		}
		timer := time.NewTimer(pollInterval(wait, result))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func BrowserAuthSessionExpiresWithin(session *AuthSession, window time.Duration, now time.Time) bool {
	if session == nil || session.AccessTokenExpiresAt == 0 {
		return true
	}
	if now.IsZero() {
		now = time.Now()
	}
	return time.Unix(session.AccessTokenExpiresAt, 0).Sub(now) <= window
}

func (r *DeviceAuthPollResult) AuthSession() *AuthSession {
	if r == nil {
		return nil
	}
	return &AuthSession{
		AccessToken:           r.AccessToken,
		RefreshToken:          r.RefreshToken,
		TokenType:             r.TokenType,
		AccessTokenExpiresAt:  r.AccessExpiresAt,
		RefreshTokenExpiresAt: r.RefreshExpiresAt,
		UserID:                r.UserID,
		WorkspaceID:           r.WorkspaceID,
		WorkspaceName:         r.WorkspaceName,
		WorkspaceRole:         r.WorkspaceRole,
		Scopes:                r.Scopes,
	}
}

func pollInterval(wait WaitForDeviceAuthOptions, result *DeviceAuthPollResult) time.Duration {
	if wait.Interval > 0 {
		return wait.Interval
	}
	if result != nil && result.IntervalSeconds > 0 {
		return time.Duration(result.IntervalSeconds) * time.Second
	}
	return defaultDevicePollInterval
}
