package local

import "github.com/scalebox-dev/agent-api-sdk/go/agentapi"

type KnowledgeSourceType = agentapi.LocalKnowledgeSourceType
type KnowledgeScope = agentapi.LocalKnowledgeScope
type KnowledgeSearchParams = agentapi.LocalKnowledgeSearchParams
type KnowledgeContextParams = agentapi.LocalKnowledgeContextParams
type KnowledgeIngestMessage = agentapi.LocalKnowledgeIngestMessage
type KnowledgeIngestWorkdirOptions = agentapi.LocalKnowledgeIngestWorkdirOptions
type KnowledgeHit = agentapi.LocalKnowledgeHit
type KnowledgeSearchResult = agentapi.LocalKnowledgeSearchResult
type KnowledgeContext = agentapi.LocalKnowledgeContext
type KnowledgeService = agentapi.LocalKnowledgeService
type KnowledgeToolRegistryOptions = agentapi.LocalKnowledgeToolRegistryOptions
type KnowledgeToolRegistry = agentapi.LocalKnowledgeToolRegistry

const (
	KnowledgeSourceTranscript  = agentapi.LocalKnowledgeSourceTranscript
	KnowledgeSourceWorkdirFile = agentapi.LocalKnowledgeSourceWorkdirFile
	KnowledgeSourceNote        = agentapi.LocalKnowledgeSourceNote
	KnowledgeSourceToolOutput  = agentapi.LocalKnowledgeSourceToolOutput
)

func CreateKnowledgeToolRegistry(service KnowledgeService, opts KnowledgeToolRegistryOptions) *agentapi.LocalKnowledgeToolRegistry {
	return agentapi.CreateLocalKnowledgeToolRegistry(service, opts)
}

func KnowledgeToolDefinition(name string) agentapi.Tool {
	return agentapi.LocalKnowledgeToolDefinition(name)
}

func FormatKnowledgeContext(context KnowledgeContext) string {
	return agentapi.FormatLocalKnowledgeContext(context)
}
