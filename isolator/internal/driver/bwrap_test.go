package driver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

func TestDiscoverBwrapReportsAvailability(t *testing.T) {
	discovery := DiscoverBwrap(context.Background())
	if discovery.Driver.Name() != "bwrap" {
		t.Fatalf("driver = %s", discovery.Driver.Name())
	}
	if !discovery.Available && len(discovery.Warnings) == 0 {
		t.Fatalf("expected unavailable discovery to include warnings")
	}
}

func TestBwrapRunWhenAvailable(t *testing.T) {
	discovery := DiscoverBwrap(context.Background())
	if !discovery.Available {
		t.Skipf("bwrap unavailable on this host: %s", strings.Join(discovery.Warnings, " "))
	}
	root := t.TempDir()
	result, err := discovery.Driver.Run(context.Background(), protocol.RunRequest{
		Command: "printf isolated > out.txt && printf ok",
		CWD:     root,
		Isolation: protocol.IsolationOptions{
			Filesystem: "workdir-readwrite",
			Network:    "allowed",
			Env:        "minimal",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Output != "ok" {
		t.Fatalf("result = %#v", result)
	}
	if !result.Isolation.Isolated || result.Isolation.Driver != "bwrap" || result.Isolation.Guarantees.Process != "pid-namespace" {
		t.Fatalf("isolation = %#v", result.Isolation)
	}
	content, err := os.ReadFile(filepath.Join(root, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "isolated" {
		t.Fatalf("content = %q", content)
	}
}
