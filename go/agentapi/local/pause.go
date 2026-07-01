package local

import "github.com/scalebox-dev/agent-api-sdk/go/agentapi"

const DefaultPauseMaxDurationMS = agentapi.DefaultLocalPauseMaxDurationMS

type PauseToolRegistryOptions = agentapi.LocalPauseToolRegistryOptions
type PauseRequest = agentapi.LocalPauseRequest
type PauseResult = agentapi.LocalPauseResult
type PauseToolRegistry = agentapi.LocalPauseToolRegistry

func CreatePauseToolRegistry(opts PauseToolRegistryOptions) *agentapi.LocalPauseToolRegistry {
	return agentapi.CreateLocalPauseToolRegistry(opts)
}

func PauseToolDefinition(name string, maxDurationMS ...int) agentapi.Tool {
	return agentapi.LocalPauseToolDefinition(name, maxDurationMS...)
}

func PauseToolInstructions(maxDurationMS ...int) string {
	return agentapi.LocalPauseToolInstructions(maxDurationMS...)
}
