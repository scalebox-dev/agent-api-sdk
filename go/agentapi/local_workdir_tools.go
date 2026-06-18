package agentapi

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type LocalWorkdirAccessMode string

const (
	LocalWorkdirAccessApproval LocalWorkdirAccessMode = "approval"
	LocalWorkdirAccessFull     LocalWorkdirAccessMode = "full"
)

type LocalWorkdirAction string

const (
	LocalWorkdirActionSummarize    LocalWorkdirAction = "summarize"
	LocalWorkdirActionList         LocalWorkdirAction = "list"
	LocalWorkdirActionSearch       LocalWorkdirAction = "search"
	LocalWorkdirActionGrep         LocalWorkdirAction = "grep"
	LocalWorkdirActionRead         LocalWorkdirAction = "read"
	LocalWorkdirActionReadLines    LocalWorkdirAction = "read_lines"
	LocalWorkdirActionContext      LocalWorkdirAction = "context"
	LocalWorkdirActionSnapshot     LocalWorkdirAction = "snapshot"
	LocalWorkdirActionClassifyPath LocalWorkdirAction = "classify_path"
	LocalWorkdirActionPreviewEdits LocalWorkdirAction = "preview_edits"
	LocalWorkdirActionApplyEdits   LocalWorkdirAction = "apply_edits"
	LocalWorkdirActionWrite        LocalWorkdirAction = "write"
	LocalWorkdirActionMkdir        LocalWorkdirAction = "mkdir"
	LocalWorkdirActionDelete       LocalWorkdirAction = "delete"
)

type LocalWorkdirExecutor interface {
	SummarizeLocalWorkdir(map[string]any) (any, error)
	ListLocalWorkdir(path string, args map[string]any) (any, error)
	SearchLocalWorkdir(map[string]any) (any, error)
	GrepLocalWorkdir(map[string]any) (any, error)
	ReadLocalWorkdir(path string, args map[string]any) (any, error)
	ReadLocalWorkdirLines(path string, args map[string]any) (any, error)
	CreateLocalWorkdirContext(map[string]any) (any, error)
	SnapshotLocalWorkdir(map[string]any) (any, error)
	ClassifyLocalWorkdirPath(path string) (any, error)
	PreviewLocalWorkdirEdits([]map[string]any) (any, error)
	ApplyLocalWorkdirEdits([]map[string]any) (any, error)
	WriteLocalWorkdir(path, content string) (any, error)
	MkdirLocalWorkdir(path string) (any, error)
	DeleteLocalWorkdir(path string) (any, error)
}

type LocalWorkdirToolRegistryOptions struct {
	AccessMode LocalWorkdirAccessMode
	ToolName   string
}

type LocalWorkdirToolHandler func(map[string]any) (map[string]any, error)

type LocalWorkdirToolRegistry struct {
	Workdir  LocalWorkdirExecutor
	Driver     *LocalWorkdirDriver
	AccessMode LocalWorkdirAccessMode
	ToolName   string
}

type LocalWorkdirDriver struct {
	Workdir  LocalWorkdirExecutor
	AccessMode LocalWorkdirAccessMode
}

func CreateLocalWorkdirToolRegistry(workdir LocalWorkdirExecutor, opts LocalWorkdirToolRegistryOptions) *LocalWorkdirToolRegistry {
	toolName := strings.TrimSpace(opts.ToolName)
	if toolName == "" {
		toolName = "local_workdir"
	}
	accessMode := opts.AccessMode
	if accessMode == "" {
		accessMode = LocalWorkdirAccessApproval
	}
	driver := &LocalWorkdirDriver{Workdir: workdir, AccessMode: accessMode}
	return &LocalWorkdirToolRegistry{
		Workdir:  workdir,
		Driver:     driver,
		AccessMode: accessMode,
		ToolName:   toolName,
	}
}

func (r *LocalWorkdirToolRegistry) Definitions() []Tool {
	return []Tool{LocalWorkdirToolDefinition(r.ToolName)}
}

func (r *LocalWorkdirToolRegistry) Handlers() map[string]LocalWorkdirToolHandler {
	return map[string]LocalWorkdirToolHandler{r.ToolName: r.Driver.Dispatch}
}

func (r *LocalWorkdirToolRegistry) Execute(name string, args map[string]any) (map[string]any, error) {
	if name != r.ToolName {
		return nil, fmt.Errorf("unknown local workdir tool: %s", name)
	}
	return r.Driver.Dispatch(args)
}

func (r *LocalWorkdirToolRegistry) RequiresApproval(name string, args map[string]any) bool {
	return name == r.ToolName && r.Driver.RequiresApproval(args)
}

func (d *LocalWorkdirDriver) Dispatch(args map[string]any) (out map[string]any, err error) {
	if d.Workdir == nil {
		return nil, fmt.Errorf("local workdir executor is required")
	}
	action, err := workdirAction(args)
	if err != nil {
		return nil, err
	}
	switch action {
	case LocalWorkdirActionSummarize:
		value, err := d.Workdir.SummarizeLocalWorkdir(args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionList:
		path, err := optionalStringArg(args, "path")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) == "" {
			path = "."
		}
		value, err := d.Workdir.ListLocalWorkdir(path, args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionSearch:
		value, err := d.Workdir.SearchLocalWorkdir(args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionGrep:
		value, err := d.Workdir.GrepLocalWorkdir(args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionRead:
		path, err := stringArg(args, "path")
		if err != nil {
			return nil, err
		}
		value, err := d.Workdir.ReadLocalWorkdir(path, args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionReadLines:
		path, err := stringArg(args, "path")
		if err != nil {
			return nil, err
		}
		value, err := d.Workdir.ReadLocalWorkdirLines(path, args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionContext:
		value, err := d.Workdir.CreateLocalWorkdirContext(args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionSnapshot:
		value, err := d.Workdir.SnapshotLocalWorkdir(args)
		return localToolResult(action, value, err)
	case LocalWorkdirActionClassifyPath:
		path, err := stringArg(args, "path")
		if err != nil {
			return nil, err
		}
		value, err := d.Workdir.ClassifyLocalWorkdirPath(path)
		return localToolResult(action, value, err)
	case LocalWorkdirActionPreviewEdits:
		edits, err := editsArg(args)
		if err != nil {
			return nil, err
		}
		value, err := d.Workdir.PreviewLocalWorkdirEdits(edits)
		return localToolResult(action, value, err)
	case LocalWorkdirActionApplyEdits:
		return d.dispatchApplyEdits(args)
	case LocalWorkdirActionWrite:
		return d.dispatchWrite(args)
	case LocalWorkdirActionMkdir:
		return d.dispatchMkdir(args)
	case LocalWorkdirActionDelete:
		return d.dispatchDelete(args)
	default:
		return nil, fmt.Errorf("unsupported local_workdir action: %s", action)
	}
}

func (d *LocalWorkdirDriver) RequiresApproval(args map[string]any) bool {
	if d.AccessMode == LocalWorkdirAccessFull {
		return false
	}
	action, err := workdirAction(args)
	if err != nil {
		return false
	}
	return mutatingLocalWorkdirActions[action]
}

func (d *LocalWorkdirDriver) dispatchApplyEdits(args map[string]any) (map[string]any, error) {
	edits, err := editsArg(args)
	if err != nil {
		return nil, err
	}
	if d.AccessMode != LocalWorkdirAccessFull {
		preview, err := d.Workdir.PreviewLocalWorkdirEdits(edits)
		if err != nil {
			return nil, err
		}
		return approvalRequired(LocalWorkdirActionApplyEdits, args, preview), nil
	}
	value, err := d.Workdir.ApplyLocalWorkdirEdits(edits)
	return localToolResult(LocalWorkdirActionApplyEdits, value, err)
}

func (d *LocalWorkdirDriver) dispatchWrite(args map[string]any) (map[string]any, error) {
	if d.AccessMode != LocalWorkdirAccessFull {
		return approvalRequired(LocalWorkdirActionWrite, args, nil), nil
	}
	path, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}
	content, err := stringArg(args, "content")
	if err != nil {
		return nil, err
	}
	value, err := d.Workdir.WriteLocalWorkdir(path, content)
	return localToolResult(LocalWorkdirActionWrite, value, err)
}

func (d *LocalWorkdirDriver) dispatchMkdir(args map[string]any) (map[string]any, error) {
	if d.AccessMode != LocalWorkdirAccessFull {
		return approvalRequired(LocalWorkdirActionMkdir, args, nil), nil
	}
	path, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}
	value, err := d.Workdir.MkdirLocalWorkdir(path)
	return localToolResult(LocalWorkdirActionMkdir, value, err)
}

func (d *LocalWorkdirDriver) dispatchDelete(args map[string]any) (map[string]any, error) {
	if d.AccessMode != LocalWorkdirAccessFull {
		return approvalRequired(LocalWorkdirActionDelete, args, nil), nil
	}
	path, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}
	value, err := d.Workdir.DeleteLocalWorkdir(path)
	return localToolResult(LocalWorkdirActionDelete, value, err)
}

func LocalWorkdirToolDefinition(name string) Tool {
	if strings.TrimSpace(name) == "" {
		name = "local_workdir"
	}
	strict := false
	return Tool{
		Type:        "function",
		Name:        name,
		Description: LocalWorkdirToolInstructions(),
		Parameters:  localWorkdirToolParameters(),
		Strict:      &strict,
	}
}

func LocalWorkdirToolInstructions() string {
	return strings.Join([]string{
		"Inspect and modify the selected local workdir through one model-facing primitive.",
		"Use action=list/search/grep/summarize/context to discover files, read/read_lines for file content, preview_edits before edits, and apply_edits/write/mkdir/delete only when mutation is intended.",
		"In approval mode, mutating actions return requires_approval with a safe preview instead of changing files. In full mode, mutating actions execute immediately.",
		"Paths are relative to the selected local workdir; never use absolute paths.",
	}, " ")
}

var localWorkdirActions = []LocalWorkdirAction{
	LocalWorkdirActionSummarize,
	LocalWorkdirActionList,
	LocalWorkdirActionSearch,
	LocalWorkdirActionGrep,
	LocalWorkdirActionRead,
	LocalWorkdirActionReadLines,
	LocalWorkdirActionContext,
	LocalWorkdirActionSnapshot,
	LocalWorkdirActionClassifyPath,
	LocalWorkdirActionPreviewEdits,
	LocalWorkdirActionApplyEdits,
	LocalWorkdirActionWrite,
	LocalWorkdirActionMkdir,
	LocalWorkdirActionDelete,
}

var mutatingLocalWorkdirActions = map[LocalWorkdirAction]bool{
	LocalWorkdirActionApplyEdits: true,
	LocalWorkdirActionWrite:      true,
	LocalWorkdirActionMkdir:      true,
	LocalWorkdirActionDelete:     true,
}

func workdirAction(args map[string]any) (LocalWorkdirAction, error) {
	value, err := stringArg(args, "action")
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, action := range localWorkdirActions {
		if string(action) == value {
			return action, nil
		}
	}
	return "", fmt.Errorf("unsupported local_workdir action: %s", value)
}

func editsArg(args map[string]any) ([]map[string]any, error) {
	if raw, ok := args["edits"]; ok {
		if values, ok := raw.([]any); ok && len(values) > 0 {
			edits := make([]map[string]any, 0, len(values))
			for _, value := range values {
				record, ok := value.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("each edit must be an object")
				}
				edit, err := editArg(record)
				if err != nil {
					return nil, err
				}
				edits = append(edits, edit)
			}
			return edits, nil
		}
		if values, ok := raw.([]map[string]any); ok && len(values) > 0 {
			edits := make([]map[string]any, 0, len(values))
			for _, value := range values {
				edit, err := editArg(value)
				if err != nil {
					return nil, err
				}
				edits = append(edits, edit)
			}
			return edits, nil
		}
	}
	if _, ok := args["path"].(string); ok && hasNumberArg(args, "startLine", "start_line") {
		edit, err := editArg(args)
		if err != nil {
			return nil, err
		}
		return []map[string]any{edit}, nil
	}
	return nil, fmt.Errorf("edits must be a non-empty array")
}

func editArg(record map[string]any) (map[string]any, error) {
	path, err := stringArg(record, "path")
	if err != nil {
		return nil, err
	}
	startLine, err := intArg(record, "startLine", "start_line")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"path": path, "start_line": startLine}
	if endLine, ok, err := optionalIntArg(record, "endLine", "end_line"); err != nil {
		return nil, err
	} else if ok {
		out["end_line"] = endLine
	}
	if replacement, ok := record["replacement"].(string); ok {
		out["replacement"] = replacement
	} else {
		out["replacement"] = ""
	}
	if expected, err := optionalStringArg(record, "expectedSha256", "expected_sha256"); err != nil {
		return nil, err
	} else if expected != "" {
		out["expected_sha256"] = expected
	}
	return out, nil
}

func localToolResult(action LocalWorkdirAction, value any, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	out := map[string]any{"ok": true, "action": string(action)}
	if value == nil {
		return out, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		out["result"] = value
		return out, nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err == nil && object != nil {
		for key, item := range object {
			out[key] = item
		}
		return out, nil
	}
	out["result"] = value
	return out, nil
}

func approvalRequired(action LocalWorkdirAction, args map[string]any, preview any) map[string]any {
	return map[string]any{
		"ok":                false,
		"action":            string(action),
		"requires_approval": true,
		"arguments":         args,
		"preview":           preview,
		"message":           fmt.Sprintf("local_workdir action %s requires approval", action),
	}
}

func stringArg(args map[string]any, key string, alternates ...string) (string, error) {
	value, ok := argValue(args, key, alternates...)
	if !ok {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return text, nil
}

func optionalStringArg(args map[string]any, key string, alternates ...string) (string, error) {
	value, ok := argValue(args, key, alternates...)
	if !ok || value == nil || value == "" {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func intArg(args map[string]any, key string, alternates ...string) (int, error) {
	value, ok := argValue(args, key, alternates...)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	out, ok := coerceInt(value)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return out, nil
}

func optionalIntArg(args map[string]any, key string, alternates ...string) (int, bool, error) {
	value, ok := argValue(args, key, alternates...)
	if !ok || value == nil {
		return 0, false, nil
	}
	out, ok := coerceInt(value)
	if !ok {
		return 0, false, fmt.Errorf("%s must be a number", key)
	}
	return out, true, nil
}

func hasNumberArg(args map[string]any, key string, alternates ...string) bool {
	value, ok := argValue(args, key, alternates...)
	if !ok {
		return false
	}
	_, ok = coerceInt(value)
	return ok
}

func argValue(args map[string]any, key string, alternates ...string) (any, bool) {
	if value, ok := args[key]; ok {
		return value, true
	}
	for _, alternate := range alternates {
		if value, ok := args[alternate]; ok {
			return value, true
		}
	}
	if options, ok := args["options"].(map[string]any); ok {
		if value, ok := options[key]; ok {
			return value, true
		}
		for _, alternate := range alternates {
			if value, ok := options[alternate]; ok {
				return value, true
			}
		}
	}
	return nil, false
}

func coerceInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		out, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(out), true
	default:
		return 0, false
	}
}

func localWorkdirToolParameters() map[string]any {
	return objectSchema(
		map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        localWorkdirActionStrings(),
				"description": "Workdir operation. Prefer summarize/list/search/grep before reading or editing. Prefer read_lines and apply_edits for source changes.",
			},
			"path":        stringSchema("Relative path. File path for read/write/delete/edit actions; directory base for list/search/grep/summarize/context/snapshot."),
			"query":       stringSchema("Path/name query for search, or optional context query."),
			"pattern":     stringSchema("Literal text pattern for grep."),
			"content":     stringSchema("Text content for write."),
			"start_line":  integerSchema("1-based start line for read_lines and edit entries."),
			"end_line":    integerSchema("1-based inclusive end line; omit or 0 for EOF when supported."),
			"replacement": stringSchema("Replacement text for simple single edit flows."),
			"edits": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "Line edits for preview_edits/apply_edits.",
				"items": objectSchema(
					map[string]any{
						"path":            stringSchema("Relative file path."),
						"start_line":      integerSchema("1-based start line."),
						"end_line":        integerSchema("1-based inclusive end line."),
						"replacement":     stringSchema("Replacement text. Empty string deletes the line range."),
						"expected_sha256": stringSchema("Optional expected SHA-256 for conflict detection."),
					},
					[]string{"path", "start_line"},
				),
			},
			"options": objectSchema(
				map[string]any{
					"recursive":           booleanSchema("List recursively."),
					"include_directories": booleanSchema("Include directories in list results."),
					"max_depth":           integerSchema("Maximum recursive list depth."),
					"limit":               integerSchema("Maximum entries or matches."),
					"max_files":           integerSchema("Maximum files to scan or package."),
					"max_bytes":           integerSchema("Maximum total bytes to read/package."),
					"max_bytes_per_file":  integerSchema("Maximum bytes per file."),
					"max_previews":        integerSchema("Maximum summary previews."),
					"include_content":     booleanSchema("Include file contents in context packages."),
					"include_summary":     booleanSchema("Include workdir summary in context packages."),
					"include_search":      booleanSchema("Include grep results in context packages when query is set."),
					"include_secrets":     booleanSchema("Include likely secret file contents in context packages."),
					"hash":                booleanSchema("Include SHA-256 hashes in snapshots."),
				},
				nil,
			),
		},
		[]string{"action"},
	)
}

func localWorkdirActionStrings() []string {
	out := make([]string, 0, len(localWorkdirActions))
	for _, action := range localWorkdirActions {
		out = append(out, string(action))
	}
	return out
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func booleanSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}
