package driver

import (
	"context"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

type Driver interface {
	Name() string
	Status(protocol.IsolationOptions) protocol.IsolationStatus
	Run(context.Context, protocol.RunRequest) (protocol.RunResult, error)
}

type Discovery struct {
	Driver    Driver
	Available bool
	Warnings  []string
}

func DirectStatus(fallback bool, opts protocol.IsolationOptions) protocol.IsolationStatus {
	requested := protocol.NormalizeIsolationOptions(opts)
	return protocol.IsolationStatus{
		Executor:  "direct",
		Driver:    "direct",
		Isolated:  false,
		Fallback:  fallback,
		Requested: requested,
		Guarantees: protocol.IsolationGuarantees{
			Filesystem: "none",
			Network:    "allowed",
			User:       "host-user",
			Process:    "host-process-tree",
			Resources:  "timeout-only",
		},
		Warnings: directWarnings(fallback, requested),
	}
}

func directWarnings(fallback bool, requested protocol.IsolationOptions) []string {
	warning := "Direct host execution has no OS-level isolation."
	if fallback {
		warning = "No local shell isolator is configured; falling back to direct host execution."
	}
	warnings := []string{warning}
	if requested.Filesystem != "host" {
		warnings = append(warnings, "Requested filesystem isolation ("+requested.Filesystem+") is not enforced by direct execution.")
	}
	if requested.Network == "blocked" {
		warnings = append(warnings, "Requested network blocking is not enforced by direct execution.")
	}
	if requested.Env == "minimal" {
		warnings = append(warnings, "Requested minimal environment is not enforced by direct execution.")
	}
	if requested.Resources.MemoryMB != nil || requested.Resources.CPUCount != nil {
		warnings = append(warnings, "Requested CPU or memory limits are not enforced by direct execution.")
	}
	return warnings
}
