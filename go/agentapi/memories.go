package agentapi

import "context"

type MemoriesService struct{ http *httpClient }

type MemorySearchParams struct {
	Query              string  `json:"query"`
	Limit              int     `json:"limit,omitempty"`
	PreviousResponseID string  `json:"previous_response_id,omitempty"`
	TenantSearch       bool    `json:"tenant_search,omitempty"`
	Lang               string  `json:"lang,omitempty"`
	SemanticWeight     float64 `json:"semantic_weight,omitempty"`
}

type MemorySearchHit struct {
	ID           string         `json:"id"`
	Score        float64        `json:"score,omitempty"`
	ThreadID     string         `json:"thread_id,omitempty"`
	CreatedAt    int64          `json:"created_at,omitempty"`
	Fact         string         `json:"fact"`
	TenantID     string         `json:"tenant_id,omitempty"`
	UserID       string         `json:"user_id,omitempty"`
	ResponseID   string         `json:"response_id,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	MetadataText string         `json:"metadata_text,omitempty"`
}

type MemorySearchModelUsage struct {
	Source       string `json:"source,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	AttemptIndex int    `json:"attempt_index,omitempty"`
	Status       string `json:"status,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}

type MemorySearchResponse struct {
	Object         string                   `json:"object"`
	Data           []MemorySearchHit        `json:"data"`
	Total          int64                    `json:"total,omitempty"`
	RewrittenQuery string                   `json:"rewritten_query,omitempty"`
	ModelUsage     []MemorySearchModelUsage `json:"model_usage,omitempty"`
}

func (s *MemoriesService) Search(ctx context.Context, params MemorySearchParams, opts ...RequestOption) (*MemorySearchResponse, error) {
	var out MemorySearchResponse
	err := s.http.requestJSON(ctx, "POST", "/v1/memories/search", params, &out, opts...)
	return &out, err
}
