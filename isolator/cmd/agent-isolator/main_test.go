package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAgentIsolatorOnceRun(t *testing.T) {
	root := t.TempDir()
	request := map[string]any{
		"id":     "req_1",
		"method": "run",
		"params": map[string]any{
			"command": "printf hello > out.txt && printf ok",
			"cwd":     root,
			"isolation": map[string]any{
				"filesystem": "workdir-readwrite",
				"network":    "blocked",
			},
		},
	}
	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "--once", "--driver=direct")
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run: %v\nstderr: %s", err, stderr.String())
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("response: %v\n%s", err, stdout.String())
	}
	if response["id"] != "req_1" {
		t.Fatalf("response = %#v", response)
	}
	result := response["result"].(map[string]any)
	if result["output"] != "ok" {
		t.Fatalf("result = %#v", result)
	}
	isolation := result["shell_isolation"].(map[string]any)
	if isolation["driver"] != "direct" || isolation["isolated"] != false {
		t.Fatalf("isolation = %#v", isolation)
	}
	content, err := os.ReadFile(filepath.Join(root, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q", content)
	}
}

func TestAgentIsolatorStatusIncludesDriverMatrix(t *testing.T) {
	request := map[string]any{"id": "status_1", "method": "status", "params": map[string]any{}}
	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "--once", "--driver=direct")
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run: %v\nstderr: %s", err, stderr.String())
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("response: %v\n%s", err, stdout.String())
	}
	result := response["result"].(map[string]any)
	drivers := result["drivers"].([]any)
	names := map[string]bool{}
	for _, item := range drivers {
		driver := item.(map[string]any)
		names[driver["name"].(string)] = true
	}
	for _, name := range []string{"direct", "bwrap", "sandbox-exec", "windows-job"} {
		if !names[name] {
			t.Fatalf("missing driver %s in %#v", name, drivers)
		}
	}
}

func TestAgentIsolatorUnsupportedPlatformDriversFailClearly(t *testing.T) {
	for _, driver := range []string{"sandbox-exec", "windows-job"} {
		if (driver == "sandbox-exec" && runtime.GOOS == "darwin") || (driver == "windows-job" && runtime.GOOS == "windows") {
			continue
		}
		cmd := exec.Command("go", "run", ".", "--once", "--driver="+driver)
		cmd.Stdin = strings.NewReader(`{"id":"status_1","method":"status","params":{}}`)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			t.Fatalf("expected %s to fail on %s", driver, runtime.GOOS)
		}
		if !strings.Contains(stderr.String(), "not available") {
			t.Fatalf("stderr = %s", stderr.String())
		}
	}
}

func TestAgentIsolatorNativeSmoke(t *testing.T) {
	if os.Getenv("AGENT_ISOLATOR_NATIVE_SMOKE") != "1" {
		t.Skip("set AGENT_ISOLATOR_NATIVE_SMOKE=1 to validate the native isolator driver on this host")
	}
	expected := expectedNativeDriver()
	if expected == "" {
		t.Skipf("no native driver expectation for %s", runtime.GOOS)
	}

	status := runIsolatorOnce(t, "auto", map[string]any{
		"id":     "native_status",
		"method": "status",
		"params": map[string]any{},
	})
	statusResult := status["result"].(map[string]any)
	if statusResult["driver"] != expected {
		t.Fatalf("auto selected driver %q, want %q; status=%s", statusResult["driver"], expected, prettyJSON(statusResult))
	}
	statusIsolation := statusResult["status"].(map[string]any)
	if statusIsolation["isolated"] != true {
		t.Fatalf("native driver did not report isolation: %s", prettyJSON(statusIsolation))
	}

	root := t.TempDir()
	request := map[string]any{
		"id":     "native_run",
		"method": "run",
		"params": map[string]any{
			"command":          nativeSmokeCommand(),
			"cwd":              root,
			"timeout_ms":       30_000,
			"max_output_bytes": 4096,
			"isolation": map[string]any{
				"filesystem": "workdir-readwrite",
				"network":    "allowed",
				"env":        "inherit",
			},
		},
	}
	run := runIsolatorOnce(t, "auto", request)
	result := run["result"].(map[string]any)
	if result["ok"] != true || !strings.Contains(result["output"].(string), "agent-isolator-smoke") {
		t.Fatalf("native run result = %s", prettyJSON(result))
	}
	runIsolation := result["shell_isolation"].(map[string]any)
	if runIsolation["driver"] != expected || runIsolation["isolated"] != true {
		t.Fatalf("native run isolation = %s", prettyJSON(runIsolation))
	}
	content, err := os.ReadFile(filepath.Join(root, "isolator-smoke.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(content)) != "agent-isolator-smoke" {
		t.Fatalf("smoke file content = %q", string(content))
	}
}

func runIsolatorOnce(t *testing.T, driver string, request map[string]any) map[string]any {
	t.Helper()
	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "--once", "--driver="+driver)
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run --driver=%s: %v\nstderr: %s\nstdout: %s", driver, err, stderr.String(), stdout.String())
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("response: %v\n%s", err, stdout.String())
	}
	if response["error"] != nil {
		t.Fatalf("response error: %s", prettyJSON(response["error"]))
	}
	return response
}

func expectedNativeDriver() string {
	if override := os.Getenv("AGENT_ISOLATOR_EXPECT_DRIVER"); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "linux":
		return "bwrap"
	case "darwin":
		return "sandbox-exec"
	case "windows":
		return "windows-job"
	default:
		return ""
	}
}

func nativeSmokeCommand() string {
	if runtime.GOOS == "windows" {
		return "echo agent-isolator-smoke> isolator-smoke.txt && type isolator-smoke.txt"
	}
	return "printf agent-isolator-smoke > isolator-smoke.txt && cat isolator-smoke.txt"
}

func prettyJSON(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
