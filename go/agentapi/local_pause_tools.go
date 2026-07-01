package agentapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const DefaultLocalPauseMaxDurationMS = 5 * 60 * 1000

type LocalPauseToolRegistryOptions struct {
	MaxDurationMS int
	ToolName      string
}

type LocalPauseRequest struct {
	DurationMS int    `json:"duration_ms"`
	Reason     string `json:"reason,omitempty"`
}

type LocalPauseResult struct {
	OK            bool   `json:"ok"`
	Tool          string `json:"tool"`
	Action        string `json:"action"`
	RequestedMS   int    `json:"requested_ms"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	ResumeMessage string `json:"resume_message,omitempty"`
}

type LocalPauseToolRegistry struct {
	MaxDurationMS int
	ToolName      string
}

func CreateLocalPauseToolRegistry(opts LocalPauseToolRegistryOptions) *LocalPauseToolRegistry {
	toolName := strings.TrimSpace(opts.ToolName)
	if toolName == "" {
		toolName = "local_pause"
	}
	maxDurationMS := opts.MaxDurationMS
	if maxDurationMS <= 0 {
		maxDurationMS = DefaultLocalPauseMaxDurationMS
	}
	return &LocalPauseToolRegistry{MaxDurationMS: maxDurationMS, ToolName: toolName}
}

func (r *LocalPauseToolRegistry) Definitions() []Tool {
	return []Tool{LocalPauseToolDefinition(r.ToolName, r.MaxDurationMS)}
}

func (r *LocalPauseToolRegistry) Execute(name string, args map[string]any) (map[string]any, error) {
	return r.ExecuteContext(context.Background(), name, args)
}

func (r *LocalPauseToolRegistry) ExecuteContext(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if name != r.ToolName {
		return nil, fmt.Errorf("unknown local pause tool: %s", name)
	}
	request, err := localPauseRequest(args, r.MaxDurationMS)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	timer := time.NewTimer(time.Duration(request.DurationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		result := LocalPauseResult{
			OK:          true,
			Tool:        r.ToolName,
			Action:      "pause",
			RequestedMS: request.DurationMS,
			ElapsedMS:   time.Since(started).Milliseconds(),
			Status:      "completed",
			Reason:      request.Reason,
		}
		return localPauseResultMap(result), nil
	}
}

func (r *LocalPauseToolRegistry) RequiresApproval(name string, args map[string]any) bool {
	return false
}

func LocalPauseToolDefinition(name string, maxDurationMS ...int) Tool {
	if strings.TrimSpace(name) == "" {
		name = "local_pause"
	}
	limit := DefaultLocalPauseMaxDurationMS
	if len(maxDurationMS) > 0 && maxDurationMS[0] > 0 {
		limit = maxDurationMS[0]
	}
	strict := false
	return Tool{
		Type:        "function",
		Name:        name,
		Description: LocalPauseToolInstructions(limit),
		Parameters:  localPauseToolParameters(limit),
		Strict:      &strict,
	}
}

func LocalPauseToolInstructions(maxDurationMS ...int) string {
	limit := DefaultLocalPauseMaxDurationMS
	if len(maxDurationMS) > 0 && maxDurationMS[0] > 0 {
		limit = maxDurationMS[0]
	}
	return strings.Join([]string{
		"Pause the local agentic workflow for a bounded amount of time, then continue automatically.",
		"Use this only when waiting for external state such as CI, deployment rollout, rate-limit cooldown, file sync, or another asynchronous process.",
		"The pause is local runtime control, not reasoning time. Keep the reason concrete.",
		fmt.Sprintf("Maximum duration: %d ms.", limit),
	}, " ")
}

func localPauseToolParameters(maxDurationMS int) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"duration_ms": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     maxDurationMS,
				"description": "How long to wait before continuing automatically, in milliseconds.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Short reason for the wait, such as the external state being awaited.",
			},
		},
		"required":             []string{"duration_ms"},
		"additionalProperties": false,
	}
}

func localPauseRequest(args map[string]any, maxDurationMS int) (LocalPauseRequest, error) {
	duration, ok := numberOption(args, "duration_ms", "durationMs")
	if !ok || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return LocalPauseRequest{}, fmt.Errorf("local_pause duration_ms must be a finite number")
	}
	durationMS := int(duration)
	if durationMS < 1 {
		return LocalPauseRequest{}, fmt.Errorf("local_pause duration_ms must be at least 1")
	}
	if durationMS > maxDurationMS {
		return LocalPauseRequest{}, fmt.Errorf("local_pause duration_ms must be <= %d", maxDurationMS)
	}
	reason := ""
	if value, ok := args["reason"].(string); ok {
		reason = strings.TrimSpace(value)
	}
	return LocalPauseRequest{DurationMS: durationMS, Reason: reason}, nil
}

func numberOption(args map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		switch value := args[key].(type) {
		case int:
			return float64(value), true
		case int64:
			return float64(value), true
		case float64:
			return value, true
		case float32:
			return float64(value), true
		case json.Number:
			out, err := value.Float64()
			return out, err == nil
		}
	}
	return 0, false
}

func localPauseResultMap(result LocalPauseResult) map[string]any {
	out := map[string]any{
		"ok":           result.OK,
		"tool":         result.Tool,
		"action":       result.Action,
		"requested_ms": result.RequestedMS,
		"elapsed_ms":   result.ElapsedMS,
		"status":       result.Status,
	}
	if result.Reason != "" {
		out["reason"] = result.Reason
	}
	if result.ResumeMessage != "" {
		out["resume_message"] = result.ResumeMessage
	}
	return out
}
