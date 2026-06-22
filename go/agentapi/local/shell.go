package local

import "github.com/scalebox-dev/agent-api-sdk/go/agentapi"

type ShellToolRegistryOptions = agentapi.LocalShellToolRegistryOptions

func CreateShellToolRegistry(workdir *Workdir, opts ShellToolRegistryOptions) (*agentapi.LocalShellToolRegistry, error) {
	if opts.CWD == "" && workdir != nil {
		opts.CWD = workdir.Root
	}
	return agentapi.CreateLocalShellToolRegistry(opts)
}
