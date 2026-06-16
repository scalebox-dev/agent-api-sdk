package agentapi

import "context"

type CatalogService struct {
	http *httpClient
	path string
}

type CatalogListResponse struct {
	Object string           `json:"object"`
	Data   []map[string]any `json:"data"`
}

func (s *CatalogService) List(ctx context.Context, opts ...RequestOption) (*CatalogListResponse, error) {
	var out CatalogListResponse
	if err := s.http.requestJSON(ctx, "GET", s.path, nil, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}
