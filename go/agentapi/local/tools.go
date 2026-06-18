package local

import "github.com/scalebox-dev/agent-api-sdk/go/agentapi"

type WorkdirToolRegistryOptions = agentapi.LocalWorkdirToolRegistryOptions

func CreateWorkdirToolRegistry(workdir *Workdir, opts WorkdirToolRegistryOptions) *agentapi.LocalWorkdirToolRegistry {
	return agentapi.CreateLocalWorkdirToolRegistry(WorkdirToolExecutor{Workdir: workdir}, opts)
}

type WorkdirToolExecutor struct {
	Workdir *Workdir
}

func (e WorkdirToolExecutor) SummarizeLocalWorkdir(args map[string]any) (any, error) {
	return e.Workdir.Summarize(SummaryParams{
		Path:        stringOption(args, "path"),
		MaxFiles:    intOption(args, "maxFiles", "max_files"),
		MaxPreviews: intOption(args, "maxPreviews", "max_previews"),
	})
}

func (e WorkdirToolExecutor) ListLocalWorkdir(path string, args map[string]any) (any, error) {
	return e.Workdir.ListEntries(path, ListOptions{
		Recursive:          boolOption(args, "recursive"),
		IncludeDirectories: boolOption(args, "includeDirectories", "include_directories"),
		MaxDepth:           intOption(args, "maxDepth", "max_depth"),
	})
}

func (e WorkdirToolExecutor) SearchLocalWorkdir(args map[string]any) (any, error) {
	return e.Workdir.SearchEntries(stringOption(args, "query"), stringOption(args, "path"), intOption(args, "limit"))
}

func (e WorkdirToolExecutor) GrepLocalWorkdir(args map[string]any) (any, error) {
	return e.Workdir.Grep(GrepParams{
		Pattern:  stringOption(args, "pattern"),
		Path:     stringOption(args, "path"),
		Limit:    intOption(args, "limit"),
		MaxFiles: intOption(args, "maxFiles", "max_files"),
	})
}

func (e WorkdirToolExecutor) ReadLocalWorkdir(path string, args map[string]any) (any, error) {
	return e.Workdir.ReadFile(path, ReadFileParams{MaxBytes: intOption(args, "maxBytes", "max_bytes")})
}

func (e WorkdirToolExecutor) ReadLocalWorkdirLines(path string, args map[string]any) (any, error) {
	return e.Workdir.ReadLines(path, ReadLinesParams{
		StartLine: intOption(args, "startLine", "start_line"),
		EndLine:   intOption(args, "endLine", "end_line"),
		MaxBytes:  intOption(args, "maxBytes", "max_bytes"),
	})
}

func (e WorkdirToolExecutor) CreateLocalWorkdirContext(args map[string]any) (any, error) {
	return CreateContextPackage(e.Workdir, ContextPackageParams{
		Path:            stringOption(args, "path"),
		Query:           stringOption(args, "query"),
		MaxFiles:        intOption(args, "maxFiles", "max_files"),
		MaxBytes:        intOption(args, "maxBytes", "max_bytes"),
		MaxBytesPerFile: intOption(args, "maxBytesPerFile", "max_bytes_per_file"),
		OmitContent:     boolOptionPresent(args, "includeContent", "include_content") && !boolOption(args, "includeContent", "include_content"),
		OmitSummary:     boolOptionPresent(args, "includeSummary", "include_summary") && !boolOption(args, "includeSummary", "include_summary"),
		IncludeSearch:   boolOption(args, "includeSearch", "include_search"),
		IncludeSecrets:  boolOption(args, "includeSecrets", "include_secrets"),
	})
}

func (e WorkdirToolExecutor) SnapshotLocalWorkdir(args map[string]any) (any, error) {
	hashPresent := boolOptionPresent(args, "hash")
	return e.Workdir.Snapshot(SnapshotParams{
		Path:            stringOption(args, "path"),
		OmitHash:        hashPresent && !boolOption(args, "hash"),
		MaxBytesPerFile: intOption(args, "maxBytesPerFile", "max_bytes_per_file"),
	})
}

func (e WorkdirToolExecutor) ClassifyLocalWorkdirPath(path string) (any, error) {
	return e.Workdir.ClassifyPath(path), nil
}

func (e WorkdirToolExecutor) PreviewLocalWorkdirEdits(edits []map[string]any) (any, error) {
	return e.Workdir.PreviewEdits(lineEdits(edits))
}

func (e WorkdirToolExecutor) ApplyLocalWorkdirEdits(edits []map[string]any) (any, error) {
	return e.Workdir.ApplyEdits(lineEdits(edits))
}

func (e WorkdirToolExecutor) WriteLocalWorkdir(path, content string) (any, error) {
	return e.Workdir.WriteFile(path, []byte(content))
}

func (e WorkdirToolExecutor) MkdirLocalWorkdir(path string) (any, error) {
	return e.Workdir.CreateDirectory(path)
}

func (e WorkdirToolExecutor) DeleteLocalWorkdir(path string) (any, error) {
	return e.Workdir.DeletePath(path)
}

func lineEdits(edits []map[string]any) []LineEdit {
	out := make([]LineEdit, 0, len(edits))
	for _, edit := range edits {
		out = append(out, LineEdit{
			Path:           stringMapValue(edit, "path"),
			StartLine:      intMapValue(edit, "start_line"),
			EndLine:        intMapValue(edit, "end_line"),
			Replacement:    stringMapValue(edit, "replacement"),
			ExpectedSHA256: stringMapValue(edit, "expected_sha256"),
		})
	}
	return out
}

func stringOption(args map[string]any, key string, alternates ...string) string {
	if value, ok := optionValue(args, key, alternates...); ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func intOption(args map[string]any, key string, alternates ...string) int {
	if value, ok := optionValue(args, key, alternates...); ok {
		return intMapValue(map[string]any{"value": value}, "value")
	}
	return 0
}

func boolOption(args map[string]any, key string, alternates ...string) bool {
	if value, ok := optionValue(args, key, alternates...); ok {
		if out, ok := value.(bool); ok {
			return out
		}
	}
	return false
}

func boolOptionPresent(args map[string]any, key string, alternates ...string) bool {
	_, ok := optionValue(args, key, alternates...)
	return ok
}

func optionValue(args map[string]any, key string, alternates ...string) (any, bool) {
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

func stringMapValue(record map[string]any, key string) string {
	if value, ok := record[key].(string); ok {
		return value
	}
	return ""
}

func intMapValue(record map[string]any, key string) int {
	switch value := record[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
