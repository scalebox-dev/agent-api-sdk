package agentapi_test

import (
	"strings"
	"testing"

	agentapi "github.com/scalebox-dev/agent-api-sdk/go/agentapi"
	"github.com/scalebox-dev/agent-api-sdk/go/agentapi/local"
)

func TestLocalWorkspaceToolDefinitionIsOneModelFacingPrimitive(t *testing.T) {
	tool := agentapi.LocalWorkspaceToolDefinition("")

	if tool.Type != "function" || tool.Name != "local_workspace" {
		t.Fatalf("tool = %#v", tool)
	}
	if tool.Strict == nil || *tool.Strict {
		t.Fatalf("strict = %#v", tool.Strict)
	}
	if got := tool.Parameters["required"]; len(got.([]string)) != 1 || got.([]string)[0] != "action" {
		t.Fatalf("required = %#v", got)
	}
	if !strings.Contains(tool.Description, "one model-facing primitive") {
		t.Fatalf("description = %q", tool.Description)
	}
}

func TestLocalWorkspaceRegistryExecutesReadActions(t *testing.T) {
	workspace := newTestWorkspace(t)
	if _, err := workspace.WriteText("README.md", "# Demo\nneedle\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkspaceToolRegistry(workspace, agentapi.LocalWorkspaceToolRegistryOptions{})

	if len(registry.Definitions()) != 1 || registry.Definitions()[0].Name != "local_workspace" {
		t.Fatalf("definitions = %#v", registry.Definitions())
	}
	if _, ok := registry.Handlers()["local_workspace"]; !ok {
		t.Fatalf("missing handler")
	}

	listed, err := registry.Execute("local_workspace", map[string]any{"action": "list", "options": map[string]any{"recursive": true}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed["ok"] != true {
		t.Fatalf("listed = %#v", listed)
	}

	grep, err := registry.Handlers()["local_workspace"](map[string]any{"action": "grep", "pattern": "needle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	matches := grep["matches"].([]any)
	if matches[0].(map[string]any)["path"] != "README.md" {
		t.Fatalf("grep = %#v", grep)
	}

	if _, err := registry.Execute("other_tool", map[string]any{"action": "list"}); err == nil || !strings.Contains(err.Error(), "unknown local workspace tool") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocalWorkspaceToolApprovalModeReturnsPreviewWithoutMutation(t *testing.T) {
	workspace := newTestWorkspace(t)
	if _, err := workspace.WriteText("notes.txt", "one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkspaceToolRegistry(workspace, agentapi.LocalWorkspaceToolRegistryOptions{AccessMode: agentapi.LocalWorkspaceAccessApproval})
	args := map[string]any{
		"action": "apply_edits",
		"edits":  []any{map[string]any{"path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"}},
	}

	if !registry.RequiresApproval("local_workspace", args) {
		t.Fatalf("expected approval")
	}
	result, err := registry.Execute("local_workspace", args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != false || result["requires_approval"] != true {
		t.Fatalf("result = %#v", result)
	}
	content, err := workspace.ReadText("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "one\ntwo\n" {
		t.Fatalf("content = %q", content)
	}
}

func TestLocalWorkspaceToolFullModeAppliesMutations(t *testing.T) {
	workspace := newTestWorkspace(t)
	if _, err := workspace.WriteText("notes.txt", "one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkspaceToolRegistry(workspace, agentapi.LocalWorkspaceToolRegistryOptions{AccessMode: agentapi.LocalWorkspaceAccessFull})

	result, err := registry.Execute(
		"local_workspace",
		map[string]any{"action": "apply_edits", "path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"},
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if registry.RequiresApproval("local_workspace", map[string]any{"action": "write", "path": "x.txt", "content": "x"}) {
		t.Fatalf("full mode should not require approval")
	}
	if result["ok"] != true {
		t.Fatalf("result = %#v", result)
	}
	content, err := workspace.ReadText("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "one\nTWO\n" {
		t.Fatalf("content = %q", content)
	}
}

func newTestWorkspace(t *testing.T) *local.Workspace {
	t.Helper()
	workspace, err := local.NewWorkspace(t.TempDir(), local.WorkspaceOptions{Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Ensure(); err != nil {
		t.Fatal(err)
	}
	return workspace
}
