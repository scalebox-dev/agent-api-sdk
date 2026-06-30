package agentapi_test

import (
	"strings"
	"testing"

	agentapi "github.com/scalebox-dev/agent-api-sdk/go/agentapi"
	"github.com/scalebox-dev/agent-api-sdk/go/agentapi/local"
)

func TestLocalWorkdirToolDefinitionIsOneModelFacingPrimitive(t *testing.T) {
	tool := agentapi.LocalWorkdirToolDefinition("")

	if tool.Type != "function" || tool.Name != "local_workdir" {
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

func TestLocalWorkdirRegistryExecutesReadActions(t *testing.T) {
	workdir := newTestWorkspace(t)
	if _, err := workdir.WriteText("README.md", "# Demo\nneedle\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkdirToolRegistry(workdir, agentapi.LocalWorkdirToolRegistryOptions{})

	if len(registry.Definitions()) != 1 || registry.Definitions()[0].Name != "local_workdir" {
		t.Fatalf("definitions = %#v", registry.Definitions())
	}
	if _, ok := registry.Handlers()["local_workdir"]; !ok {
		t.Fatalf("missing handler")
	}

	listed, err := registry.Execute("local_workdir", map[string]any{"action": "list", "options": map[string]any{"recursive": true}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed["ok"] != true {
		t.Fatalf("listed = %#v", listed)
	}

	grep, err := registry.Handlers()["local_workdir"](map[string]any{"action": "grep", "pattern": "needle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	matches := grep["matches"].([]any)
	if matches[0].(map[string]any)["path"] != "README.md" {
		t.Fatalf("grep = %#v", grep)
	}

	fileGrep, err := registry.Execute("local_workdir", map[string]any{"action": "grep", "path": "README.md", "pattern": "needle"})
	if err != nil {
		t.Fatalf("file grep: %v", err)
	}
	fileMatches := fileGrep["matches"].([]any)
	if fileGrep["ok"] != true || len(fileMatches) != 1 || fileMatches[0].(map[string]any)["path"] != "README.md" {
		t.Fatalf("file grep = %#v", fileGrep)
	}

	missing, err := registry.Execute("local_workdir", map[string]any{"action": "grep", "path": "missing.txt", "pattern": "needle"})
	if err != nil {
		t.Fatalf("missing grep should return tool error, got %v", err)
	}
	if missing["ok"] != false || missing["action"] != "grep" {
		t.Fatalf("missing grep = %#v", missing)
	}

	if _, err := registry.Execute("other_tool", map[string]any{"action": "list"}); err == nil || !strings.Contains(err.Error(), "unknown local workdir tool") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocalWorkdirToolApprovalModeReturnsPreviewWithoutMutation(t *testing.T) {
	workdir := newTestWorkspace(t)
	if _, err := workdir.WriteText("notes.txt", "one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkdirToolRegistry(workdir, agentapi.LocalWorkdirToolRegistryOptions{AccessMode: agentapi.LocalWorkdirAccessApproval})
	args := map[string]any{
		"action": "apply_edits",
		"edits":  []any{map[string]any{"path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"}},
	}

	if !registry.RequiresApproval("local_workdir", args) {
		t.Fatalf("expected approval")
	}
	result, err := registry.Execute("local_workdir", args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != false || result["requires_approval"] != true {
		t.Fatalf("result = %#v", result)
	}
	content, err := workdir.ReadText("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "one\ntwo\n" {
		t.Fatalf("content = %q", content)
	}
}

func TestLocalWorkdirToolFullModeAppliesMutations(t *testing.T) {
	workdir := newTestWorkspace(t)
	if _, err := workdir.WriteText("notes.txt", "one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	registry := local.CreateWorkdirToolRegistry(workdir, agentapi.LocalWorkdirToolRegistryOptions{AccessMode: agentapi.LocalWorkdirAccessFull})

	result, err := registry.Execute(
		"local_workdir",
		map[string]any{"action": "apply_edits", "path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"},
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if registry.RequiresApproval("local_workdir", map[string]any{"action": "write", "path": "x.txt", "content": "x"}) {
		t.Fatalf("full mode should not require approval")
	}
	if result["ok"] != true {
		t.Fatalf("result = %#v", result)
	}
	if result["edit_count"] != float64(1) && result["edit_count"] != 1 {
		t.Fatalf("result = %#v", result)
	}
	changedFiles, ok := result["changed_files"].([]any)
	if !ok || len(changedFiles) != 1 || changedFiles[0] != "notes.txt" {
		t.Fatalf("result = %#v", result)
	}
	if _, ok := result["backups"]; ok {
		t.Fatalf("result leaked backups: %#v", result)
	}
	content, err := workdir.ReadText("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "one\nTWO\n" {
		t.Fatalf("content = %q", content)
	}
}

func newTestWorkspace(t *testing.T) *local.Workdir {
	t.Helper()
	workdir, err := local.NewWorkdir(t.TempDir(), local.WorkdirOptions{Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workdir.Ensure(); err != nil {
		t.Fatal(err)
	}
	return workdir
}
