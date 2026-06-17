package agentapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type contextAwareBody struct {
	ctx context.Context
	r   *strings.Reader
}

func (b *contextAwareBody) Read(p []byte) (int, error) {
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	default:
		return b.r.Read(p)
	}
}

func (b *contextAwareBody) Close() error { return nil }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestClient(fn roundTripFunc) *Client {
	return NewClient(&ClientOptions{
		APIKey:     "sk-test",
		BaseURL:    "https://api.test",
		HTTPClient: &http.Client{Transport: fn},
		MaxRetries: -1,
	})
}

func TestResponseCreateAddsOutputTextAndHeaders(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := req.Header.Get("User-Agent"); got != userAgent {
			t.Fatalf("User-Agent = %q", got)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		return jsonResponse(200, `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`), nil
	})
	resp, err := client.Responses.Create(context.Background(), ResponseCreateParams{Input: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.OutputText != "hello" {
		t.Fatalf("OutputText = %q", resp.OutputText)
	}
}

func TestRawUploadUsesOctetStream(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut || req.URL.Path != "/v1/volumes/vol/files/a.txt" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("Content-Type = %q", got)
		}
		raw, _ := io.ReadAll(req.Body)
		if string(raw) != "abc" {
			t.Fatalf("body = %q", string(raw))
		}
		return jsonResponse(200, `{"path":"a.txt","size":3}`), nil
	})
	if _, err := client.Volumes.WriteFile(context.Background(), "vol", "a.txt", []byte("abc")); err != nil {
		t.Fatal(err)
	}
}

func TestDeviceAuthFlowStartsPollsAndWaits(t *testing.T) {
	var calls []string
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.Path)
		switch req.URL.Path {
		case "/v1/auth/device/start":
			raw, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(raw), "Agent CLI") {
				t.Fatalf("start body = %s", raw)
			}
			return jsonResponse(200, `{"device_code":"dev_secret","user_code":"ABCD1234","verification_uri":"https://www.example.test/auth/device","verification_uri_complete":"https://www.example.test/auth/device?user_code=ABCD1234","expires_at":4102444800,"interval_seconds":1}`), nil
		case "/v1/auth/device/poll":
			raw, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(raw), "dev_secret") {
				t.Fatalf("poll body = %s", raw)
			}
			polls := 0
			for _, call := range calls {
				if call == "/v1/auth/device/poll" {
					polls++
				}
			}
			if polls == 1 {
				return jsonResponse(200, `{"status":"pending","message":"authorization pending","interval_seconds":1,"expires_at":4102444800}`), nil
			}
			return jsonResponse(200, `{"status":"approved","access_token":"jwt","refresh_token":"refresh","access_token_expires_at":4102441200,"user_id":"user_1","workspace_id":"wrk_1","workspace_role":"owner","scopes":["responses:create"]}`), nil
		default:
			t.Fatalf("unexpected request path: %s", req.URL.Path)
			return nil, nil
		}
	})

	started, err := client.Auth.StartDeviceAuth(context.Background(), StartDeviceAuthParams{ClientName: "Agent CLI"})
	if err != nil {
		t.Fatal(err)
	}
	if started.DeviceCode != "dev_secret" {
		t.Fatalf("DeviceCode = %q", started.DeviceCode)
	}
	session, err := client.Auth.WaitForDeviceAuth(context.Background(), PollDeviceAuthParams{DeviceCode: started.DeviceCode}, WaitForDeviceAuthOptions{
		Interval: time.Millisecond,
		Timeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != "jwt" || session.WorkspaceID != "wrk_1" {
		t.Fatalf("session = %#v", session)
	}
}

func TestStreamPreservesWhitespaceDeltas(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"###\"}\n\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\n" +
			"data: {\"type\":\"response.completed\",\"response_id\":\"resp_1\"}\n\n"
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: &contextAwareBody{ctx: req.Context(), r: strings.NewReader(body)}}, nil
	})
	stream, err := client.Responses.CreateStream(context.Background(), ResponseCreateParams{Input: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var text string
	for stream.Next() {
		text += stream.Event().Delta
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if text != "### " {
		t.Fatalf("stream text = %q", text)
	}
}

func TestSupportedRoutesManifest(t *testing.T) {
	routes := SupportedRoutes()
	if len(routes) != 49 {
		t.Fatalf("route count = %d", len(routes))
	}
	seen := map[string]bool{}
	for _, route := range routes {
		if route.Symbol == "" || route.Method == "" || route.Path == "" {
			t.Fatalf("incomplete route: %#v", route)
		}
		if seen[route.Symbol] {
			t.Fatalf("duplicate route symbol: %s", route.Symbol)
		}
		seen[route.Symbol] = true
	}
}
