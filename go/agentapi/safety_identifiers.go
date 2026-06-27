package agentapi

import "context"

type SafetyIdentifiersService struct {
	http *httpClient
}

type SafetyIdentifier struct {
	Object           string `json:"object"`
	WorkspaceID      string `json:"workspace_id"`
	SafetyIdentifier string `json:"safety_identifier"`
	CreatedByUserID  string `json:"created_by_user_id"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type ListSafetyIdentifiersParams struct {
	PageSize  int
	PageToken string
}

type ListSafetyIdentifiersResponse struct {
	Object string             `json:"object"`
	Data   []SafetyIdentifier `json:"data"`
}

func (s *SafetyIdentifiersService) List(ctx context.Context, params ListSafetyIdentifiersParams, opts ...RequestOption) (*ListSafetyIdentifiersResponse, error) {
	var out ListSafetyIdentifiersResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/safety_identifiers"+buildQuery(map[string]any{
		"page_size":  params.PageSize,
		"page_token": params.PageToken,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SafetyIdentifiersService) Lookup(ctx context.Context, safetyIdentifier string, opts ...RequestOption) (*SafetyIdentifier, error) {
	var out SafetyIdentifier
	err := s.http.requestJSON(ctx, "GET", "/v1/safety_identifiers/lookup"+buildQuery(map[string]any{
		"safety_identifier": safetyIdentifier,
	}), nil, &out, opts...)
	return &out, err
}
