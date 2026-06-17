package local

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

type ContextPackageParams struct {
	Path            string
	Include         []string
	Exclude         []string
	Query           string
	MaxFiles        int
	MaxBytes        int
	MaxBytesPerFile int
	PreviewBytes    int
	OmitContent     bool
	OmitHashes      bool
	OmitSummary     bool
	IncludeSearch   bool
	IncludeSecrets  bool
}

type ContextFile struct {
	Path              string          `json:"path"`
	Size              int64           `json:"size"`
	SHA256            string          `json:"sha256,omitempty"`
	MimeType          string          `json:"mime_type,omitempty"`
	Sensitivity       PathSensitivity `json:"sensitivity"`
	SensitivityReason string          `json:"sensitivity_reason,omitempty"`
	Content           string          `json:"content,omitempty"`
	ContentBase64     string          `json:"content_base64,omitempty"`
	Encoding          string          `json:"encoding,omitempty"`
	Truncated         bool            `json:"truncated,omitempty"`
	OmittedReason     string          `json:"omitted_reason,omitempty"`
}

type ContextManifest struct {
	Object          string        `json:"object"`
	Root            string        `json:"root"`
	WorkspaceName   string        `json:"workspace_name"`
	GeneratedAtUnix int64         `json:"generated_at_unix"`
	BasePath        string        `json:"base_path"`
	FileCount       int           `json:"file_count"`
	TotalBytes      int64         `json:"total_bytes"`
	IncludedBytes   int           `json:"included_bytes"`
	Truncated       bool          `json:"truncated"`
	Files           []ContextFile `json:"files"`
	Summary         *Summary      `json:"summary,omitempty"`
	Search          *GrepResponse `json:"search,omitempty"`
}

func CreateContextPackage(workspace *Workspace, params ContextPackageParams) (*ContextManifest, error) {
	basePath := firstNonEmpty(params.Path, ".")
	maxFiles := firstPositiveInt(params.MaxFiles, 80)
	maxBytes := firstPositiveInt(params.MaxBytes, 256*1024)
	maxBytesPerFile := firstPositiveInt(params.MaxBytesPerFile, 32*1024)
	previewBytes := firstPositiveInt(params.PreviewBytes, maxBytesPerFile)
	includeContent := !params.OmitContent
	includeHashes := !params.OmitHashes
	includeSummary := !params.OmitSummary
	stats, err := workspace.List(basePath, ListOptions{Recursive: true})
	if err != nil {
		return nil, err
	}
	var files []FileStat
	for _, item := range stats {
		if item.Type == FileTypeFile && matchesPathFilters(item.Path, params.Include, params.Exclude) {
			files = append(files, item)
		}
	}
	out := &ContextManifest{Object: "local_context_manifest", Root: workspace.Root, WorkspaceName: workspace.Name, GeneratedAtUnix: time.Now().Unix(), BasePath: basePath, FileCount: len(files), Truncated: len(files) > maxFiles}
	for _, item := range files {
		out.TotalBytes += item.Size
	}
	for _, item := range files {
		if len(out.Files) >= maxFiles {
			break
		}
		sensitivity := ClassifyPathSensitivity(item.Path)
		packaged := ContextFile{Path: item.Path, Size: item.Size, Sensitivity: sensitivity.Sensitivity, SensitivityReason: sensitivity.Reason}
		if !params.IncludeSecrets && sensitivity.Sensitivity == SensitivitySecret {
			packaged.OmittedReason = "secret_path"
			out.Files = append(out.Files, packaged)
			continue
		}
		if item.Size > int64(maxBytesPerFile) {
			packaged.OmittedReason = "file_too_large"
			out.Truncated = true
			out.Files = append(out.Files, packaged)
			continue
		}
		if out.IncludedBytes >= maxBytes {
			packaged.OmittedReason = "package_budget_exceeded"
			out.Truncated = true
			out.Files = append(out.Files, packaged)
			continue
		}
		readBudget := minInt(previewBytes, minInt(maxBytesPerFile, maxBytes-out.IncludedBytes))
		if readBudget <= 0 {
			packaged.OmittedReason = "package_budget_exceeded"
			out.Truncated = true
			out.Files = append(out.Files, packaged)
			continue
		}
		delivered, err := workspace.ReadFile(item.Path, ReadFileParams{MaxBytes: readBudget})
		if err != nil {
			return nil, err
		}
		packaged.MimeType = delivered.MimeType
		packaged.Encoding = delivered.Encoding
		packaged.Truncated = delivered.Truncated
		if includeContent {
			packaged.Content = delivered.Content
			packaged.ContentBase64 = delivered.ContentBase64
		}
		if delivered.Encoding == "text" {
			out.IncludedBytes += len([]byte(delivered.Content))
		} else {
			raw, _ := base64.StdEncoding.DecodeString(delivered.ContentBase64)
			out.IncludedBytes += len(raw)
		}
		if includeHashes {
			raw, err := workspace.Files.ReadBytes(item.Path)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(raw)
			packaged.SHA256 = hex.EncodeToString(sum[:])
		}
		if delivered.Truncated {
			out.Truncated = true
		}
		out.Files = append(out.Files, packaged)
	}
	if includeSummary {
		out.Summary, err = workspace.Summarize(SummaryParams{Path: basePath, MaxFiles: maxFiles, PreviewBytes: previewBytes, Ignore: ignoreGlobs(params.Exclude)})
		if err != nil {
			return nil, err
		}
	}
	if params.IncludeSearch && params.Query != "" {
		out.Search, err = workspace.Grep(GrepParams{Pattern: params.Query, Path: basePath, Limit: maxFiles, MaxBytesPerFile: maxBytesPerFile, Ignore: ignoreGlobs(params.Exclude)})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
