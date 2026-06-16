package agentapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestStreamPreservesWhitespaceDeltas(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"###\"}\n\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\n" +
			"data: {\"type\":\"response.completed\",\"response_id\":\"resp_1\"}\n\n"
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
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
	if len(routes) != 47 {
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
