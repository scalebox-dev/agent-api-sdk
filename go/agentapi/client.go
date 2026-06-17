package agentapi

import "context"

type Client struct {
	Responses *ResponsesService
	Agent     *ResponsesService
	Models    *CatalogService
	Presets   *CatalogService
	Tools     *CatalogService
	Volumes   *VolumesService
	Skills    *SkillsService
	Auth      *AuthService

	http *httpClient
}

func NewClient(opts *ClientOptions) *Client {
	h := newHTTPClient(opts)
	return &Client{
		Responses: &ResponsesService{http: h, path: "/v1/responses"},
		Agent:     &ResponsesService{http: h, path: "/v1/agent"},
		Models:    &CatalogService{http: h, path: "/v1/models"},
		Presets:   &CatalogService{http: h, path: "/v1/presets"},
		Tools:     &CatalogService{http: h, path: "/v1/tools"},
		Volumes:   &VolumesService{http: h},
		Skills:    &SkillsService{http: h},
		Auth:      &AuthService{http: h},
		http:      h,
	}
}

func (c *Client) StartDeviceAuth(ctx context.Context, params StartDeviceAuthParams, opts ...RequestOption) (*DeviceAuthStart, error) {
	return c.Auth.StartDeviceAuth(ctx, params, opts...)
}

func (c *Client) PollDeviceAuth(ctx context.Context, params PollDeviceAuthParams, opts ...RequestOption) (*DeviceAuthPollResult, error) {
	return c.Auth.PollDeviceAuth(ctx, params, opts...)
}

func (c *Client) WaitForDeviceAuth(ctx context.Context, params PollDeviceAuthParams, wait WaitForDeviceAuthOptions, opts ...RequestOption) (*AuthSession, error) {
	return c.Auth.WaitForDeviceAuth(ctx, params, wait, opts...)
}
