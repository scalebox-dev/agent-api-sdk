//go:build !windows

package driver

import (
	"context"
	"runtime"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

type WindowsJob struct{}

func DiscoverWindowsJob(context.Context) Discovery {
	return Discovery{Driver: WindowsJob{}, Available: false, Warnings: []string{"windows-job driver is only supported on Windows."}}
}

func (WindowsJob) Name() string {
	return "windows-job"
}

func (WindowsJob) Status(opts protocol.IsolationOptions) protocol.IsolationStatus {
	requested := protocol.NormalizeIsolationOptions(opts)
	return protocol.IsolationStatus{
		Executor:  "isolator",
		Driver:    "windows-job",
		Isolated:  false,
		Fallback:  false,
		Requested: requested,
		Guarantees: protocol.IsolationGuarantees{
			Filesystem: "none",
			Network:    "allowed",
			User:       "host-user",
			Process:    "host-process-tree",
			Resources:  "none",
		},
		Warnings: []string{"windows-job driver is unavailable on " + runtime.GOOS + "."},
	}
}

func (WindowsJob) Run(context.Context, protocol.RunRequest) (protocol.RunResult, error) {
	return protocol.RunResult{}, errUnsupported("windows-job")
}
