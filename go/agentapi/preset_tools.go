package agentapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type UnknownPresetToolBehavior string

const (
	UnknownPresetToolStub  UnknownPresetToolBehavior = "stub"
	UnknownPresetToolOmit  UnknownPresetToolBehavior = "omit"
	UnknownPresetToolError UnknownPresetToolBehavior = "error"
)

type PresetPolicy struct {
	PlanModePreference string   `json:"plan_mode_preference,omitempty"`
	SubAgentPreference string   `json:"sub_agent_preference,omitempty"`
	AllowedTools       []string `json:"allowed_tools,omitempty"`
	MaxSteps           int      `json:"max_steps,omitempty"`
}

type PresetCatalogItem struct {
	Preset          string         `json:"preset"`
	PromptVersion   string         `json:"prompt_version,omitempty"`
	PresetMetadata  map[string]any `json:"preset_metadata,omitempty"`
	Policy          *PresetPolicy  `json:"policy,omitempty"`
	MaxOutputTokens int            `json:"max_output_tokens,omitempty"`
	DefaultModel    string         `json:"default_model,omitempty"`
	ModelChain      []string       `json:"model_chain,omitempty"`
}

type PublicTool struct {
	Object           string         `json:"object,omitempty"`
	Name             string         `json:"name"`
	Type             string         `json:"type,omitempty"`
	Description      string         `json:"description,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	MaxTokensPerPage int            `json:"max_tokens_per_page,omitempty"`
	Version          string         `json:"version,omitempty"`
}

type ResolvePresetToolsOptions struct {
	Preset            string
	Tools             []Tool
	Presets           []PresetCatalogItem
	ToolCatalog       []PublicTool
	UnknownPresetTool UnknownPresetToolBehavior
}

type ResolvePresetToolsResult struct {
	Preset           PresetCatalogItem
	Tools            []Tool
	MissingToolNames []string
}

// ResolvePresetTools fetches preset/tool catalogs, resolves the preset's allowed tools,
// and appends caller/client tools. Use this for hybrid apps that need local function
// tools while preserving preset-provided server tools.
func ResolvePresetTools(ctx context.Context, client *Client, opts ResolvePresetToolsOptions, reqOpts ...RequestOption) (*ResolvePresetToolsResult, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if opts.Presets == nil {
		resp, err := client.Presets.List(ctx, reqOpts...)
		if err != nil {
			return nil, err
		}
		if err := convertCatalogRows(resp.Data, &opts.Presets); err != nil {
			return nil, err
		}
	}
	if opts.ToolCatalog == nil {
		resp, err := client.Tools.List(ctx, reqOpts...)
		if err != nil {
			return nil, err
		}
		if err := convertCatalogRows(resp.Data, &opts.ToolCatalog); err != nil {
			return nil, err
		}
	}
	return ResolvePresetToolsFromCatalog(opts)
}

func ResolvePresetToolsFromCatalog(opts ResolvePresetToolsOptions) (*ResolvePresetToolsResult, error) {
	presetID := strings.TrimSpace(opts.Preset)
	if presetID == "" {
		return nil, fmt.Errorf("preset is required")
	}
	var preset *PresetCatalogItem
	for i := range opts.Presets {
		if opts.Presets[i].Preset == presetID {
			preset = &opts.Presets[i]
			break
		}
	}
	if preset == nil {
		return nil, fmt.Errorf("preset not found: %s", presetID)
	}

	catalogByName := make(map[string]PublicTool, len(opts.ToolCatalog))
	for _, tool := range opts.ToolCatalog {
		name := strings.TrimSpace(tool.Name)
		if name != "" {
			catalogByName[name] = tool
		}
	}

	unknown := opts.UnknownPresetTool
	if unknown == "" {
		unknown = UnknownPresetToolStub
	}
	var missing []string
	var presetTools []Tool
	if preset.Policy != nil {
		for _, name := range preset.Policy.AllowedTools {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			if catalogTool, ok := catalogByName[trimmed]; ok {
				presetTools = append(presetTools, PublicToolToRequestTool(catalogTool))
				continue
			}
			missing = append(missing, trimmed)
			switch unknown {
			case UnknownPresetToolError:
				return nil, fmt.Errorf("preset tool not found in catalog: %s", trimmed)
			case UnknownPresetToolOmit:
				continue
			default:
				presetTools = append(presetTools, Tool{Name: trimmed})
			}
		}
	}

	merged, err := MergeTools(presetTools, opts.Tools)
	if err != nil {
		return nil, err
	}
	return &ResolvePresetToolsResult{
		Preset:           *preset,
		Tools:            merged,
		MissingToolNames: missing,
	}, nil
}

func MergeTools(groups ...[]Tool) ([]Tool, error) {
	var out []Tool
	for _, group := range groups {
		for _, tool := range group {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				return nil, fmt.Errorf("tools[].name is required")
			}
			tool.Name = name
			out = append(out, tool)
		}
	}
	if err := validateUniqueToolNames(out); err != nil {
		return nil, err
	}
	return out, nil
}

func PublicToolToRequestTool(tool PublicTool) Tool {
	return Tool{
		Type:             tool.Type,
		Name:             tool.Name,
		Description:      tool.Description,
		Parameters:       tool.Parameters,
		MaxTokens:        tool.MaxTokens,
		MaxTokensPerPage: tool.MaxTokensPerPage,
		Version:          tool.Version,
	}
}

func convertCatalogRows[T any](rows []map[string]any, out *[]T) error {
	raw, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
