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

func TestVolumeAssetHelpersNormalizePrivateImageTargets(t *testing.T) {
	if got := NormalizeVolumeAssetPath("/agent-volume/reports/chart.png?cache=1"); got != "reports/chart.png" {
		t.Fatalf("NormalizeVolumeAssetPath = %q", got)
	}
	if got := NormalizeVolumeAssetPath("/reports/chart.svg#figure"); got != "reports/chart.svg" {
		t.Fatalf("NormalizeVolumeAssetPath = %q", got)
	}
	if got := NormalizeVolumeAssetPath("https://example.test/chart.png"); got != "" {
		t.Fatalf("external target normalized to %q", got)
	}
	if got := NormalizeVolumeAssetPath("../secret.png"); got != "" {
		t.Fatalf("unsafe target normalized to %q", got)
	}
	if !IsSupportedVolumeImagePath("/reports/chart.svg") {
		t.Fatal("expected svg path to be supported")
	}
	if IsSupportedVolumeImagePath("/reports/table.csv") {
		t.Fatal("did not expect csv path to be supported")
	}
	if !IsSupportedVolumeImageContentType("image/svg+xml; charset=utf-8") {
		t.Fatal("expected svg content type to be supported")
	}
	if IsSupportedVolumeImageContentType("text/html") {
		t.Fatal("did not expect html content type to be supported")
	}
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

func TestResponseCreateRejectsDuplicateToolNames(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})
	_, err := client.Responses.Create(context.Background(), ResponseCreateParams{
		Input: "hi",
		Tools: []Tool{
			{Name: "smart_web_search"},
			{Name: "smart_web_search", Type: "search"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate tools[].name: smart_web_search") {
		t.Fatalf("error = %v", err)
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

func TestRefreshBrowserSessionUsesRefreshTokenCookieAndExpiryHelper(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/auth/refresh" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Cookie"); got != "agent_api_refresh=refresh%20original" {
			t.Fatalf("Cookie = %q", got)
		}
		return jsonResponse(200, `{"access_token":"jwt_next","refresh_token":"refresh_next","access_token_expires_at":4102441200,"user_id":"user_1","workspace_id":"wrk_1","workspace_role":"owner","scopes":["responses:create"]}`), nil
	})

	session, err := client.RefreshBrowserSession(context.Background(), RefreshBrowserSessionParams{RefreshToken: "refresh original"})
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != "jwt_next" {
		t.Fatalf("AccessToken = %q", session.AccessToken)
	}
	if !BrowserAuthSessionExpiresWithin(&AuthSession{AccessTokenExpiresAt: 100}, 60*time.Second, time.Unix(41, 0)) {
		t.Fatal("expected session to be inside refresh window")
	}
	if BrowserAuthSessionExpiresWithin(&AuthSession{AccessTokenExpiresAt: 100}, 60*time.Second, time.Unix(39, 0)) {
		t.Fatal("expected session to be outside refresh window")
	}
}

func TestResolvePresetToolsFetchesDefaultsAndAppendsCallerTools(t *testing.T) {
	var calls []string
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.Path)
		switch req.URL.Path {
		case "/v1/presets":
			return jsonResponse(200, `{"object":"list","data":[{"preset":"pro-search","policy":{"allowed_tools":["smart_web_search","fetch_url"]}}]}`), nil
		case "/v1/tools":
			return jsonResponse(200, `{"object":"list","data":[{"object":"tool","name":"smart_web_search","type":"search","description":"Search broadly","max_tokens":4096,"max_tokens_per_page":2048},{"object":"tool","name":"fetch_url","type":"url_reader"}]}`), nil
		default:
			t.Fatalf("unexpected request path: %s", req.URL.Path)
			return nil, nil
		}
	})

	resolved, err := ResolvePresetTools(context.Background(), client, ResolvePresetToolsOptions{
		Preset: "pro-search",
		Tools: []Tool{{
			Type:        "function",
			Name:        "local_workdir",
			Description: "Operate on the local workdir.",
			Parameters:  map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "/v1/presets,/v1/tools" {
		t.Fatalf("calls = %#v", calls)
	}
	if resolved.Preset.Preset != "pro-search" {
		t.Fatalf("preset = %#v", resolved.Preset)
	}
	names := []string{resolved.Tools[0].Name, resolved.Tools[1].Name, resolved.Tools[2].Name}
	if strings.Join(names, ",") != "smart_web_search,fetch_url,local_workdir" {
		t.Fatalf("tool names = %#v", names)
	}
	if resolved.Tools[0].Type != "search" || resolved.Tools[0].MaxTokens != 4096 {
		t.Fatalf("first tool = %#v", resolved.Tools[0])
	}
	if resolved.Tools[2].Type != "function" {
		t.Fatalf("local tool = %#v", resolved.Tools[2])
	}
}

func TestResolvePresetToolsFromCatalogRejectsDuplicateToolNamesByDefault(t *testing.T) {
	_, err := ResolvePresetToolsFromCatalog(ResolvePresetToolsOptions{
		Preset: "pro-search",
		Presets: []PresetCatalogItem{{
			Preset: "pro-search",
			Policy: &PresetPolicy{AllowedTools: []string{"smart_web_search", "fetch_url"}},
		}},
		ToolCatalog: []PublicTool{{Name: "smart_web_search", Type: "search"}},
		Tools:       []Tool{{Name: "smart_web_search", Type: "search", MaxTokens: 128}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate tools[].name: smart_web_search") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolvePresetToolsFromCatalogCanFailClosed(t *testing.T) {
	_, err := ResolvePresetToolsFromCatalog(ResolvePresetToolsOptions{
		Preset: "pro-search",
		Presets: []PresetCatalogItem{{
			Preset: "pro-search",
			Policy: &PresetPolicy{AllowedTools: []string{"missing_tool"}},
		}},
		UnknownPresetTool: UnknownPresetToolError,
	})
	if err == nil || !strings.Contains(err.Error(), "preset tool not found in catalog: missing_tool") {
		t.Fatalf("error = %v", err)
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
