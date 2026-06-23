package driver

import (
	"context"
	"runtime"
	"testing"
)

func TestPlatformDriverDiscoveryReportsUnsupportedHosts(t *testing.T) {
	sandboxExec := DiscoverSandboxExec(context.Background())
	if sandboxExec.Driver.Name() != "sandbox-exec" {
		t.Fatalf("sandbox-exec driver = %s", sandboxExec.Driver.Name())
	}
	if runtime.GOOS != "darwin" && sandboxExec.Available {
		t.Fatalf("sandbox-exec should not be available on %s", runtime.GOOS)
	}

	windowsJob := DiscoverWindowsJob(context.Background())
	if windowsJob.Driver.Name() != "windows-job" {
		t.Fatalf("windows-job driver = %s", windowsJob.Driver.Name())
	}
	if runtime.GOOS != "windows" && windowsJob.Available {
		t.Fatalf("windows-job should not be available on %s", runtime.GOOS)
	}
}
