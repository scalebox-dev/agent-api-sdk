package agentapi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentapi "github.com/scalebox-dev/agent-api-sdk/go/agentapi"
	"github.com/scalebox-dev/agent-api-sdk/go/agentapi/local"
)

func TestLocalShellToolDefinitionIsOneModelFacingPrimitive(t *testing.T) {
	tool := agentapi.LocalShellToolDefinition("host_command", agentapi.LocalShellToolPresentationOptions{
		AccessMode:     agentapi.LocalShellAccessFull,
		CWD:            "/tmp/example",
		Shell:          "bash",
		Platform:       "linux",
		TimeoutMS:      1000,
		MaxOutputBytes: 2048,
	})

	if tool.Type != "function" || tool.Name != "host_command" {
		t.Fatalf("tool = %#v", tool)
	}
	if tool.Strict == nil || *tool.Strict {
		t.Fatalf("strict = %#v", tool.Strict)
	}
	if !strings.Contains(tool.Description, "platform=linux") ||
		!strings.Contains(tool.Description, "shell=bash") ||
		!strings.Contains(tool.Description, "access_mode=full") ||
		!strings.Contains(tool.Description, "not a filesystem sandbox") {
		t.Fatalf("description = %q", tool.Description)
	}
}

func TestLocalShellRegistryApprovalModeDoesNotExecute(t *testing.T) {
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{CWD: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	if len(registry.Definitions()) != 1 || registry.Definitions()[0].Name != "local_shell" {
		t.Fatalf("definitions = %#v", registry.Definitions())
	}
	if _, ok := registry.Handlers()["local_shell"]; !ok {
		t.Fatalf("missing handler")
	}
	result, err := registry.Execute("local_shell", map[string]any{"command": "printf shell-ready"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != false || result["requires_approval"] != true || result["command"] != "printf shell-ready" {
		t.Fatalf("result = %#v", result)
	}
	if !registry.RequiresApproval("local_shell", nil) {
		t.Fatalf("expected approval")
	}
}

func TestLocalShellFullModeExecutesInConfiguredCWD(t *testing.T) {
	root := t.TempDir()
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        root,
		AccessMode: agentapi.LocalShellAccessFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute("local_shell", map[string]any{
		"command":     "printf shell-output > result.txt && printf done",
		"description": "Write result file",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != true || result["exit_code"].(float64) != 0 || !strings.Contains(result["output"].(string), "done") {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "shell-output" {
		t.Fatalf("content = %q", content)
	}
}

func TestLocalShellRejectsWorkdirTraversal(t *testing.T) {
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        t.TempDir(),
		AccessMode: agentapi.LocalShellAccessFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Execute("local_shell", map[string]any{"command": "pwd", "workdir": ".."}); err == nil || !strings.Contains(err.Error(), "workdir must stay inside") {
		t.Fatalf("err = %v", err)
	}
}
