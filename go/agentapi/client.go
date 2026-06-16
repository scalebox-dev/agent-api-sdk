package agentapi

type Client struct {
	Responses *ResponsesService
	Agent     *ResponsesService
	Models    *CatalogService
	Presets   *CatalogService
	Tools     *CatalogService
	Volumes   *VolumesService
	Skills    *SkillsService

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
		http:      h,
	}
}
