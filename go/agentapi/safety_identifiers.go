package agentapi

import "context"

type SafetyIdentifiersService struct {
	http *httpClient
}

type SafetyIdentifierPartition struct {
	Object           string `json:"object"`
	WorkspaceID      string `json:"workspace_id"`
	SafetyIdentifier string `json:"safety_identifier"`
	OwnerUserID      string `json:"owner_user_id"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type ListSafetyIdentifierPartitionsParams struct {
	OwnerUserID string
	Status      string
}

type ListSafetyIdentifierPartitionsResponse struct {
	Object string                      `json:"object"`
	Data   []SafetyIdentifierPartition `json:"data"`
}

func (s *SafetyIdentifiersService) List(ctx context.Context, params ListSafetyIdentifierPartitionsParams, opts ...RequestOption) (*ListSafetyIdentifierPartitionsResponse, error) {
	var out ListSafetyIdentifierPartitionsResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/safety_identifiers"+buildQuery(map[string]any{
		"owner_user_id": params.OwnerUserID,
		"status":        params.Status,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SafetyIdentifiersService) Lookup(ctx context.Context, safetyIdentifier string, opts ...RequestOption) (*SafetyIdentifierPartition, error) {
	var out SafetyIdentifierPartition
	err := s.http.requestJSON(ctx, "GET", "/v1/safety_identifiers/lookup"+buildQuery(map[string]any{
		"safety_identifier": safetyIdentifier,
	}), nil, &out, opts...)
	return &out, err
}
