package agentapi_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
		!strings.Contains(tool.Description, "isolation_driver=direct") ||
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
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["driver"] != "direct" || isolation["isolated"] != false {
		t.Fatalf("isolation = %#v", isolation)
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
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["driver"] != "direct" || isolation["isolated"] != false || isolation["fallback"] != false {
		t.Fatalf("isolation = %#v", isolation)
	}
	content, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "shell-output" {
		t.Fatalf("content = %q", content)
	}
}

func TestLocalShellNoneIsolationIsExplicitDirectHostExecution(t *testing.T) {
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        t.TempDir(),
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute("local_shell", map[string]any{"command": "printf direct"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["executor"] != "direct" || isolation["driver"] != "direct" || isolation["isolated"] != false || isolation["fallback"] != false {
		t.Fatalf("isolation = %#v", isolation)
	}
}

func TestLocalShellAutoIsolationFallsBackToDirectExecutor(t *testing.T) {
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        t.TempDir(),
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(registry.Definitions()[0].Description, "fallback=true") {
		t.Fatalf("description = %q", registry.Definitions()[0].Description)
	}
	result, err := registry.Execute("local_shell", map[string]any{"command": "printf isolated-fallback"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["executor"] != "direct" || isolation["driver"] != "direct" || isolation["isolated"] != false || isolation["fallback"] != true {
		t.Fatalf("isolation = %#v", isolation)
	}
}

func TestLocalShellIsolationOptionsReportWithoutDirectEnforcement(t *testing.T) {
	memory := 256
	cpu := 1
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        t.TempDir(),
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationAuto,
		IsolationOptions: agentapi.LocalShellIsolationOptions{
			Filesystem: "workdir-readwrite",
			Network:    "blocked",
			Env:        "minimal",
			Resources: agentapi.LocalShellIsolationResourceOptions{
				MemoryMB: &memory,
				CPUCount: &cpu,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	description := registry.Definitions()[0].Description
	if !strings.Contains(description, "filesystem=workdir-readwrite") ||
		!strings.Contains(description, "network=blocked") ||
		!strings.Contains(description, "memory_mb=256") {
		t.Fatalf("description = %q", description)
	}
	result, err := registry.Execute("local_shell", map[string]any{"command": "printf options"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	isolation := result["shell_isolation"].(map[string]any)
	requested := isolation["requested"].(map[string]any)
	resources := requested["resources"].(map[string]any)
	guarantees := isolation["guarantees"].(map[string]any)
	if requested["filesystem"] != "workdir-readwrite" || requested["network"] != "blocked" || requested["env"] != "minimal" {
		t.Fatalf("requested = %#v", requested)
	}
	if resources["memoryMb"].(float64) != 256 || resources["cpuCount"].(float64) != 1 {
		t.Fatalf("resources = %#v", resources)
	}
	if guarantees["filesystem"] != "none" || guarantees["network"] != "allowed" {
		t.Fatalf("guarantees = %#v", guarantees)
	}
	if !strings.Contains(strings.Join(stringSlice(isolation["warnings"]), " "), "not enforced by direct execution") {
		t.Fatalf("warnings = %#v", isolation["warnings"])
	}
}

func TestLocalShellRequiredIsolationRejectsMissingIsolatingRunner(t *testing.T) {
	if _, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationRequired,
	}); err == nil || !strings.Contains(err.Error(), "agent-isolator") {
		t.Fatalf("err = %v", err)
	}
	if _, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationRequired,
		Runner:     noIsolationRunner{},
	}); err == nil || !strings.Contains(err.Error(), "does not report isolation") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocalShellRequiredIsolationCanRunThroughAgentIsolatorProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	root := t.TempDir()
	registry, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:        root,
		AccessMode: agentapi.LocalShellAccessFull,
		Isolation:  agentapi.LocalShellIsolationRequired,
		IsolationOptions: agentapi.LocalShellIsolationOptions{
			Filesystem: "workdir-readwrite",
			Network:    "allowed",
			Env:        "inherit",
		},
		UseIsolator: true,
		IsolatorOptions: agentapi.IsolatorLocalShellRunnerOptions{
			ExecutablePath: fakeAgentIsolator(t),
			Driver:         "fake",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(registry.Definitions()[0].Description, "isolation_driver=fake-isolator") {
		t.Fatalf("description = %q", registry.Definitions()[0].Description)
	}
	result, err := registry.Execute("local_shell", map[string]any{"command": "printf through-isolator"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != true || result["output"] != "through-isolator" {
		t.Fatalf("result = %#v", result)
	}
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["executor"] != "isolator" || isolation["driver"] != "fake-isolator" || isolation["isolated"] != true {
		t.Fatalf("isolation = %#v", isolation)
	}
}

func TestLocalShellRequiredIsolationFailsClosedWhenAgentIsolatorUnavailable(t *testing.T) {
	if _, err := local.CreateShellToolRegistry(nil, agentapi.LocalShellToolRegistryOptions{
		CWD:         t.TempDir(),
		AccessMode:  agentapi.LocalShellAccessFull,
		Isolation:   agentapi.LocalShellIsolationRequired,
		UseIsolator: true,
		IsolatorOptions: agentapi.IsolatorLocalShellRunnerOptions{
			ExecutablePath: filepath.Join(t.TempDir(), "missing-agent-isolator"),
		},
	}); err == nil {
		t.Fatalf("expected required isolation to fail closed")
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

type noIsolationRunner struct{}

func (noIsolationRunner) Run(_ context.Context, _ agentapi.LocalShellRequest) (agentapi.LocalShellResult, error) {
	return agentapi.LocalShellResult{}, nil
}

func fakeAgentIsolator(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent-isolator")
	script := `#!/bin/sh
payload=$(cat)
status='{"executor":"isolator","driver":"fake-isolator","isolated":true,"fallback":false,"requested":{"filesystem":"workdir-readwrite","network":"allowed","env":"inherit","resources":{}},"guarantees":{"filesystem":"workdir-mounted","network":"allowed","user":"namespace-user","process":"pid-namespace","resources":"timeout-only"},"warnings":[]}'
case "$payload" in
  *'"method":"status"'*)
    printf '{"id":"status","result":{"version":"fake","driver":"fake-isolator","status":%s,"drivers":[]}}' "$status"
    ;;
  *'"method":"run"'*)
    printf '{"id":"run","result":{"ok":true,"action":"run","command":"printf through-isolator","cwd":"/tmp","exit_code":0,"stdout":"through-isolator","stderr":"","output":"through-isolator","duration_ms":1,"timed_out":false,"truncated":false,"shell_isolation":%s}}' "$status"
    ;;
  *)
    printf '{"error":{"code":"unknown_method","message":"unknown method"}}'
    exit 1
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
