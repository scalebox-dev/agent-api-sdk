package agentapi

import "encoding/json"

type ErrorInfo struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type Usage struct {
	InputTokens         int            `json:"input_tokens,omitempty"`
	OutputTokens        int            `json:"output_tokens,omitempty"`
	TotalTokens         int            `json:"total_tokens,omitempty"`
	InputTokensDetails  map[string]int `json:"input_tokens_details,omitempty"`
	OutputTokensDetails map[string]int `json:"output_tokens_details,omitempty"`
}

type Metadata map[string]any

type ContentPart struct {
	Type        string           `json:"type"`
	Text        string           `json:"text,omitempty"`
	ImageURL    string           `json:"image_url,omitempty"`
	MimeType    string           `json:"mime_type,omitempty"`
	Filename    string           `json:"filename,omitempty"`
	DocumentURL string           `json:"document_url,omitempty"`
	Annotations []map[string]any `json:"annotations,omitempty"`
	Logprobs    []any            `json:"logprobs,omitempty"`
}

type InputMessage struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type FunctionCallInput struct {
	Type             string `json:"type"`
	CallID           string `json:"call_id"`
	Name             string `json:"name"`
	Arguments        string `json:"arguments"`
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

type FunctionCallOutputInput struct {
	Type             string `json:"type"`
	CallID           string `json:"call_id"`
	Output           string `json:"output"`
	Name             string `json:"name,omitempty"`
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

type ReasoningConfig struct {
	Effort string `json:"effort,omitempty"`
}

type ResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema map[string]any `json:"json_schema,omitempty"`
}

type MemoryOptions struct {
	Enabled bool `json:"enabled,omitempty"`
	Read    bool `json:"read,omitempty"`
	Write   bool `json:"write,omitempty"`
}

type CallerContext struct {
	Timezone string `json:"timezone,omitempty"`
	Locale   string `json:"locale,omitempty"`
	Locality string `json:"locality,omitempty"`
	Extra    any    `json:"extra,omitempty"`
}

type SkillReference struct {
	SkillID string `json:"skill_id"`
	Branch  string `json:"branch,omitempty"`
}

type LocalSkillDescriptor struct {
	LocalSkillID      string         `json:"local_skill_id"`
	SkillRef          string         `json:"skill_ref,omitempty"`
	Name              string         `json:"name,omitempty"`
	Description       string         `json:"description,omitempty"`
	RootHint          string         `json:"root_hint,omitempty"`
	Digest            string         `json:"digest,omitempty"`
	Manifest          string         `json:"manifest,omitempty"`
	ManifestTruncated bool           `json:"manifest_truncated,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type Tool struct {
	Type             string         `json:"type,omitempty"`
	Name             string         `json:"name,omitempty"`
	Description      string         `json:"description,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
	Strict           *bool          `json:"strict,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	MaxTokensPerPage int            `json:"max_tokens_per_page,omitempty"`
	Version          string         `json:"version,omitempty"`
	Arguments        map[string]any `json:"arguments,omitempty"`
}

type ResponseCreateParams struct {
	Input              any                    `json:"input"`
	Instructions       string                 `json:"instructions,omitempty"`
	LanguagePreference string                 `json:"language_preference,omitempty"`
	CallerContext      *CallerContext         `json:"caller_context,omitempty"`
	Model              string                 `json:"model,omitempty"`
	Models             []string               `json:"models,omitempty"`
	ModelRouting       string                 `json:"model_routing,omitempty"`
	RoutingStrategy    string                 `json:"routing_strategy,omitempty"`
	Preset             string                 `json:"preset,omitempty"`
	MaxOutputTokens    int                    `json:"max_output_tokens,omitempty"`
	MaxSteps           int                    `json:"max_steps,omitempty"`
	Reasoning          *ReasoningConfig       `json:"reasoning,omitempty"`
	ResponseFormat     *ResponseFormat        `json:"response_format,omitempty"`
	Tools              []Tool                 `json:"tools,omitempty"`
	ToolChoice         any                    `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool                  `json:"parallel_tool_calls,omitempty"`
	Metadata           Metadata               `json:"metadata,omitempty"`
	Store              *bool                  `json:"store,omitempty"`
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	VolumeID           string                 `json:"volume_id,omitempty"`
	PreferredSites     []string               `json:"preferred_sites,omitempty"`
	Skills             []SkillReference       `json:"skills,omitempty"`
	LocalSkills        []LocalSkillDescriptor `json:"local_skills,omitempty"`
	PromptCacheKey     string                 `json:"prompt_cache_key,omitempty"`
	Memory             *MemoryOptions         `json:"memory,omitempty"`
	PlanModePreference string                 `json:"plan_mode_preference,omitempty"`
	SubAgentPreference string                 `json:"sub_agent_preference,omitempty"`
	SafetyIdentifier   string                 `json:"safety_identifier,omitempty"`
	User               string                 `json:"user,omitempty"`
	Stream             bool                   `json:"stream,omitempty"`
}

type OutputItem struct {
	Type             string          `json:"type"`
	ID               string          `json:"id,omitempty"`
	Status           string          `json:"status,omitempty"`
	Role             string          `json:"role,omitempty"`
	Content          []ContentPart   `json:"content,omitempty"`
	Queries          []string        `json:"queries,omitempty"`
	Results          []SearchResult  `json:"results,omitempty"`
	Contents         []URLContent    `json:"contents,omitempty"`
	Name             string          `json:"name,omitempty"`
	CallID           string          `json:"call_id,omitempty"`
	Arguments        string          `json:"arguments,omitempty"`
	ThoughtSignature string          `json:"thought_signature,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

type SearchResult struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source,omitempty"`
	Date        string `json:"date,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
}

type URLContent struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type AgentResponse struct {
	ID                 string           `json:"id"`
	Object             string           `json:"object"`
	CreatedAt          int64            `json:"created_at"`
	CompletedAt        *int64           `json:"completed_at,omitempty"`
	Status             string           `json:"status"`
	Model              string           `json:"model"`
	Output             []OutputItem     `json:"output"`
	OutputText         string           `json:"output_text,omitempty"`
	Usage              *Usage           `json:"usage,omitempty"`
	Error              *ErrorInfo       `json:"error,omitempty"`
	Metadata           Metadata         `json:"metadata,omitempty"`
	Instructions       *string          `json:"instructions,omitempty"`
	Tools              []Tool           `json:"tools,omitempty"`
	ToolChoice         any              `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool            `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID *string          `json:"previous_response_id,omitempty"`
	ParentResponseID   *string          `json:"parent_response_id,omitempty"`
	RootResponseID     *string          `json:"root_response_id,omitempty"`
	PromptCacheKey     *string          `json:"prompt_cache_key,omitempty"`
	Store              *bool            `json:"store,omitempty"`
	Background         *bool            `json:"background,omitempty"`
	UserID             string           `json:"user_id,omitempty"`
	ToolResults        []ToolInvocation `json:"tool_results,omitempty"`
	Plan               any              `json:"plan,omitempty"`
	SafetyIdentifier   string           `json:"safety_identifier,omitempty"`
}

type ToolInvocation struct {
	ID                      string         `json:"id,omitempty"`
	ToolCallID              string         `json:"tool_call_id,omitempty"`
	ToolName                string         `json:"tool_name,omitempty"`
	Status                  string         `json:"status,omitempty"`
	Error                   *ErrorInfo     `json:"error,omitempty"`
	Metadata                map[string]any `json:"metadata,omitempty"`
	ResponseSummary         string         `json:"response_summary,omitempty"`
	ResponseSummaryMimeType string         `json:"response_summary_mime_type,omitempty"`
}

type ResponseListItem struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"created_at"`
	CompletedAt      *int64 `json:"completed_at,omitempty"`
	Model            string `json:"model,omitempty"`
	Preset           string `json:"preset,omitempty"`
	InputPreview     string `json:"input_preview,omitempty"`
	RootResponseID   string `json:"root_response_id,omitempty"`
	Background       *bool  `json:"background,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	SafetyIdentifier string `json:"safety_identifier,omitempty"`
}

type ListResponsesResponse struct {
	Object        string             `json:"object"`
	Data          []ResponseListItem `json:"data"`
	HasMore       bool               `json:"has_more"`
	NextPageToken string             `json:"next_page_token,omitempty"`
}

type ResponseChildItem struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"created_at"`
	CompletedAt    *int64 `json:"completed_at,omitempty"`
	RootResponseID string `json:"root_response_id,omitempty"`
	Model          string `json:"model,omitempty"`
}

type ListChildrenResponse struct {
	Object string              `json:"object"`
	Data   []ResponseChildItem `json:"data"`
}

type ListEventsResponse struct {
	Data []ResponseStreamEvent `json:"data"`
}

type CancelResponse struct {
	Interrupted bool `json:"interrupted"`
}

type ResponseStreamEvent struct {
	Type           string          `json:"type"`
	SequenceNumber int64           `json:"sequence_number,omitempty"`
	ResponseID     string          `json:"response_id,omitempty"`
	Delta          string          `json:"delta,omitempty"`
	Text           string          `json:"text,omitempty"`
	OutputIndex    int             `json:"output_index,omitempty"`
	Item           json.RawMessage `json:"item,omitempty"`
	Usage          *Usage          `json:"usage,omitempty"`
	Error          *ErrorInfo      `json:"error,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

type ListParams struct {
	Limit     int
	PageToken string
	UserID    string
}
