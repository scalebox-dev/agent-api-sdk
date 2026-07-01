package agentapi

import (
	"context"
	"strings"
	"testing"
)

func TestLocalPauseRegistryExposesBoundedWaitPrimitive(t *testing.T) {
	registry := CreateLocalPauseToolRegistry(LocalPauseToolRegistryOptions{MaxDurationMS: 100})
	definitions := registry.Definitions()
	if len(definitions) != 1 || definitions[0].Name != "local_pause" {
		t.Fatalf("definitions = %#v", definitions)
	}
	if definitions[0].Parameters["properties"].(map[string]any)["duration_ms"].(map[string]any)["maximum"] != 100 {
		t.Fatalf("duration maximum = %#v", definitions[0].Parameters)
	}
	if registry.RequiresApproval("local_pause", nil) {
		t.Fatalf("local_pause should not require approval")
	}

	result, err := registry.Execute("local_pause", map[string]any{"duration_ms": 1, "reason": "short wait"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result["ok"] != true || result["tool"] != "local_pause" || result["action"] != "pause" || result["status"] != "completed" {
		t.Fatalf("result = %#v", result)
	}
	if result["reason"] != "short wait" || result["requested_ms"] != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestLocalPauseRegistryCanBeCanceledByContext(t *testing.T) {
	registry := CreateLocalPauseToolRegistry(LocalPauseToolRegistryOptions{MaxDurationMS: 1000})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := registry.ExecuteContext(ctx, "local_pause", map[string]any{"duration_ms": 1000})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocalPauseToolDefinitionCanBeRenamed(t *testing.T) {
	tool := LocalPauseToolDefinition("pause_here", 123)
	if tool.Name != "pause_here" {
		t.Fatalf("name = %s", tool.Name)
	}
	if tool.Parameters["properties"].(map[string]any)["duration_ms"].(map[string]any)["maximum"] != 123 {
		t.Fatalf("parameters = %#v", tool.Parameters)
	}
}
