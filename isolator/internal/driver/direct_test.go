package driver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

func TestDirectRunReportsRequestedOptionsWithoutIsolation(t *testing.T) {
	root := t.TempDir()
	memory := 256
	cpu := 1
	result, err := Direct{}.Run(context.Background(), protocol.RunRequest{
		Command: "printf direct",
		CWD:     root,
		Isolation: protocol.IsolationOptions{
			Filesystem: "workdir-readwrite",
			Network:    "blocked",
			Env:        "minimal",
			Resources: protocol.IsolationResourceOptions{
				MemoryMB: &memory,
				CPUCount: &cpu,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Output != "direct" {
		t.Fatalf("result = %#v", result)
	}
	if result.Isolation.Driver != "direct" || result.Isolation.Isolated || result.Isolation.Guarantees.Filesystem != "none" {
		t.Fatalf("isolation = %#v", result.Isolation)
	}
	if result.Isolation.Requested.Filesystem != "workdir-readwrite" || result.Isolation.Requested.Network != "blocked" {
		t.Fatalf("requested = %#v", result.Isolation.Requested)
	}
	if !strings.Contains(strings.Join(result.Isolation.Warnings, " "), "not enforced by direct execution") {
		t.Fatalf("warnings = %#v", result.Isolation.Warnings)
	}
}

func TestDirectRunContainsRelativeWorkdir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Direct{}.Run(context.Background(), protocol.RunRequest{
		Command: "printf scoped > out.txt",
		CWD:     root,
		Workdir: "child",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(root, "child", "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "scoped" {
		t.Fatalf("content = %q", content)
	}
	direct := Direct{}
	if _, err := direct.Run(context.Background(), protocol.RunRequest{Command: "pwd", CWD: root, Workdir: ".."}); err == nil {
		t.Fatalf("expected traversal error")
	}
}
