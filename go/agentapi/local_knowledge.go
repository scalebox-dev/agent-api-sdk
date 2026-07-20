package agentapi

import (
	"fmt"
	"strings"
)

type LocalKnowledgeSourceType string

const (
	LocalKnowledgeSourceTranscript  LocalKnowledgeSourceType = "transcript"
	LocalKnowledgeSourceWorkdirFile LocalKnowledgeSourceType = "workdir_file"
	LocalKnowledgeSourceNote        LocalKnowledgeSourceType = "note"
	LocalKnowledgeSourceToolOutput  LocalKnowledgeSourceType = "tool_output"
)

type LocalKnowledgeScope struct {
	ConversationID string   `json:"conversationId,omitempty"`
	WorkspaceID    string   `json:"workspaceId,omitempty"`
	Profile        string   `json:"profile,omitempty"`
	Workdir        string   `json:"workdir,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

type LocalKnowledgeRetentionPolicy struct {
	TranscriptTTLSeconds int `json:"transcriptTtlSeconds,omitempty"`
	WorkdirTTLSeconds    int `json:"workdirTtlSeconds,omitempty"`
	MaxBytes             int `json:"maxBytes,omitempty"`
	MaxTranscriptSources int `json:"maxTranscriptSources,omitempty"`
	MaxWorkdirSources    int `json:"maxWorkdirSources,omitempty"`
	DeletedTTLSeconds    int `json:"deletedTtlSeconds,omitempty"`
}

type LocalKnowledgeRetrievalPolicy struct {
	DefaultLimit                int    `json:"defaultLimit,omitempty"`
	MaxLimit                    int    `json:"maxLimit,omitempty"`
	DefaultContextBytes         int    `json:"defaultContextBytes,omitempty"`
	MaxContextBytes             int    `json:"maxContextBytes,omitempty"`
	ScopeMode                   string `json:"scopeMode,omitempty"`
	IncludeConversationSiblings bool   `json:"includeConversationSiblings,omitempty"`
}

type LocalKnowledgeIngestionPolicy struct {
	MaxTranscriptBytes  int  `json:"maxTranscriptBytes,omitempty"`
	MaxWorkdirFiles     int  `json:"maxWorkdirFiles,omitempty"`
	MaxWorkdirFileBytes int  `json:"maxWorkdirFileBytes,omitempty"`
	MaxChunkBytes       int  `json:"maxChunkBytes,omitempty"`
	IncludeWorkdir      bool `json:"includeWorkdir,omitempty"`
	IncludeTranscripts  bool `json:"includeTranscripts,omitempty"`
}

type LocalKnowledgePolicy struct {
	Enabled   *bool                          `json:"enabled,omitempty"`
	Retention *LocalKnowledgeRetentionPolicy `json:"retention,omitempty"`
	Retrieval *LocalKnowledgeRetrievalPolicy `json:"retrieval,omitempty"`
	Ingestion *LocalKnowledgeIngestionPolicy `json:"ingestion,omitempty"`
}

type LocalKnowledgeSearchParams struct {
	Query          string               `json:"query"`
	Limit          int                  `json:"limit,omitempty"`
	Scope          *LocalKnowledgeScope `json:"scope,omitempty"`
	ConversationID string               `json:"conversationId,omitempty"`
	Workdir        string               `json:"workdir,omitempty"`
}

type LocalKnowledgeContextParams struct {
	LocalKnowledgeSearchParams
	MaxBytes int `json:"maxBytes,omitempty"`
}

type LocalKnowledgeIngestMessage struct {
	ConversationID string               `json:"conversationId"`
	MessageID      string               `json:"messageId"`
	Role           string               `json:"role"`
	Kind           string               `json:"kind,omitempty"`
	Text           string               `json:"text"`
	Scope          *LocalKnowledgeScope `json:"scope,omitempty"`
}

type LocalKnowledgeIngestWorkdirOptions struct {
	Root            string               `json:"root"`
	MaxFiles        int                  `json:"maxFiles,omitempty"`
	MaxBytesPerFile int                  `json:"maxBytesPerFile,omitempty"`
	Scope           *LocalKnowledgeScope `json:"scope,omitempty"`
}

type LocalKnowledgeHit struct {
	ID         string                   `json:"id"`
	SourceType LocalKnowledgeSourceType `json:"sourceType"`
	SourceURI  string                   `json:"sourceUri"`
	Title      string                   `json:"title,omitempty"`
	Text       string                   `json:"text"`
	Score      float64                  `json:"score,omitempty"`
	UpdatedAt  int64                    `json:"updatedAt,omitempty"`
	Metadata   map[string]any           `json:"metadata,omitempty"`
}

type LocalKnowledgeSearchResult struct {
	Object string              `json:"object"`
	Data   []LocalKnowledgeHit `json:"data"`
}

type LocalKnowledgeContext struct {
	Hits []LocalKnowledgeHit `json:"hits"`
	Text string              `json:"text"`
}

type LocalKnowledgeSourceStats struct {
	Sources int `json:"sources"`
	Chunks  int `json:"chunks"`
	Bytes   int `json:"bytes"`
}

type LocalKnowledgeStats struct {
	Object          string                                                 `json:"object"`
	Sources         int                                                    `json:"sources"`
	Chunks          int                                                    `json:"chunks"`
	Bytes           int                                                    `json:"bytes"`
	DeletedSources  int                                                    `json:"deletedSources"`
	OldestIndexedAt int64                                                  `json:"oldestIndexedAt,omitempty"`
	NewestIndexedAt int64                                                  `json:"newestIndexedAt,omitempty"`
	BySourceType    map[LocalKnowledgeSourceType]LocalKnowledgeSourceStats `json:"bySourceType,omitempty"`
}

type LocalKnowledgePruneParams struct {
	Policy *LocalKnowledgePolicy `json:"policy,omitempty"`
	Scope  *LocalKnowledgeScope  `json:"scope,omitempty"`
	DryRun bool                  `json:"dryRun,omitempty"`
}

type LocalKnowledgePruneResult struct {
	Object         string `json:"object"`
	DryRun         bool   `json:"dryRun,omitempty"`
	DeletedSources int    `json:"deletedSources"`
	DeletedChunks  int    `json:"deletedChunks"`
	ReclaimedBytes int    `json:"reclaimedBytes"`
}

type LocalKnowledgeForgetParams struct {
	ConversationID string                   `json:"conversationId,omitempty"`
	WorkspaceID    string                   `json:"workspaceId,omitempty"`
	Profile        string                   `json:"profile,omitempty"`
	Workdir        string                   `json:"workdir,omitempty"`
	SourceURI      string                   `json:"sourceUri,omitempty"`
	SourceType     LocalKnowledgeSourceType `json:"sourceType,omitempty"`
}

type LocalKnowledgeService interface {
	SearchLocalKnowledge(LocalKnowledgeSearchParams) (LocalKnowledgeSearchResult, error)
	CreateLocalKnowledgeContext(LocalKnowledgeContextParams) (*LocalKnowledgeContext, error)
}

type LocalKnowledgeIngester interface {
	IngestLocalKnowledgeMessage(LocalKnowledgeIngestMessage) error
	IngestLocalKnowledgeWorkdir(LocalKnowledgeIngestWorkdirOptions) error
}

type LocalKnowledgeLifecycleManager interface {
	ForgetLocalKnowledge(LocalKnowledgeForgetParams) error
	PruneLocalKnowledge(LocalKnowledgePruneParams) (LocalKnowledgePruneResult, error)
	LocalKnowledgeStats(*LocalKnowledgeScope) (LocalKnowledgeStats, error)
}

type LocalKnowledgeToolRegistryOptions struct {
	ToolName string
}

type LocalKnowledgeToolHandler func(map[string]any) (map[string]any, error)

type LocalKnowledgeToolRegistry struct {
	Service  LocalKnowledgeService
	ToolName string
}

func CreateLocalKnowledgeToolRegistry(service LocalKnowledgeService, opts LocalKnowledgeToolRegistryOptions) *LocalKnowledgeToolRegistry {
	toolName := strings.TrimSpace(opts.ToolName)
	if toolName == "" {
		toolName = "local_knowledge"
	}
	return &LocalKnowledgeToolRegistry{Service: service, ToolName: toolName}
}

func (r *LocalKnowledgeToolRegistry) Definitions() []Tool {
	return []Tool{LocalKnowledgeToolDefinition(r.ToolName)}
}

func (r *LocalKnowledgeToolRegistry) Handlers() map[string]LocalKnowledgeToolHandler {
	return map[string]LocalKnowledgeToolHandler{
		r.ToolName: func(args map[string]any) (map[string]any, error) {
			return r.Execute(r.ToolName, args)
		},
	}
}

func (r *LocalKnowledgeToolRegistry) Execute(name string, args map[string]any) (map[string]any, error) {
	if name != r.ToolName {
		return nil, fmt.Errorf("unknown local knowledge tool: %s", name)
	}
	if r.Service == nil {
		return nil, fmt.Errorf("local knowledge service is required")
	}
	action := "search"
	if raw, ok := args["action"].(string); ok && strings.TrimSpace(raw) != "" {
		action = strings.ToLower(strings.TrimSpace(raw))
	}
	if action != "search" {
		return localKnowledgeErrorResult(action, fmt.Sprintf("unsupported local_knowledge action: %s", action)), nil
	}
	query, err := stringArg(args, "query")
	if err != nil {
		return localKnowledgeErrorResult(action, "query is required"), nil
	}
	limit := 0
	if value, ok, err := optionalIntArg(args, "limit"); err != nil {
		return localKnowledgeErrorResult(action, "limit must be a number"), nil
	} else if ok {
		limit = value
	}
	result, err := r.Service.SearchLocalKnowledge(LocalKnowledgeSearchParams{
		Query: query,
		Limit: limit,
	})
	if err != nil {
		return localKnowledgeErrorResult(action, err.Error()), nil
	}
	return map[string]any{
		"object": "local_knowledge_result",
		"action": action,
		"result": result,
	}, nil
}

func LocalKnowledgeToolDefinition(name string) Tool {
	if strings.TrimSpace(name) == "" {
		name = "local_knowledge"
	}
	strict := false
	return Tool{
		Type:        "function",
		Name:        name,
		Description: LocalKnowledgeToolInstructions(),
		Parameters:  localKnowledgeToolParameters(),
		Strict:      &strict,
	}
}

func LocalKnowledgeToolInstructions() string {
	return strings.Join([]string{
		"Search the local knowledge index for durable local context from prior transcript messages and indexed workdir files.",
		"Use this when the current task may depend on local project history, prior decisions, repo conventions, or nearby documentation.",
		"The index is local and may be incomplete; use local_workdir to verify exact current file contents before editing.",
	}, " ")
}

func FormatLocalKnowledgeContext(context LocalKnowledgeContext) string {
	if len(context.Hits) == 0 {
		return ""
	}
	return strings.Join([]string{
		"Local knowledge follows. It is retrieved from local transcripts and indexed local files; treat it as contextual hints and verify exact file contents with local_workdir when precision matters.",
		context.Text,
	}, "\n")
}

func localKnowledgeToolParameters() map[string]any {
	return objectSchema(
		map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"search"},
				"description": "Local knowledge operation.",
			},
			"query": stringSchema("Natural language or keyword query."),
			"limit": integerSchema("Maximum number of hits."),
		},
		[]string{"action", "query"},
	)
}

func localKnowledgeErrorResult(action, message string) map[string]any {
	return map[string]any{
		"object": "local_knowledge_result",
		"action": action,
		"error":  map[string]any{"message": message},
	}
}
