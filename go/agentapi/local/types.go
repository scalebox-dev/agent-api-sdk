package local

import "time"

type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
	FileTypeOther     FileType = "other"
)

type PathSensitivity string

const (
	SensitivityNormal    PathSensitivity = "normal"
	SensitivitySensitive PathSensitivity = "sensitive"
	SensitivitySecret    PathSensitivity = "secret"
)

type AppDirs struct {
	Home   string
	Data   string
	Config string
	Cache  string
	Logs   string
	Temp   string
}

type FileStat struct {
	Path       string
	FullPath   string
	Type       FileType
	Size       int64
	ModifiedAt time.Time
}

type Entry struct {
	Path           string `json:"path"`
	IsDir          bool   `json:"is_dir"`
	Size           int64  `json:"size"`
	ModifiedAtUnix int64  `json:"modified_at_unix,omitempty"`
}

type EntryList struct {
	Object  string  `json:"object"`
	Entries []Entry `json:"entries"`
}

type FileDeliver struct {
	Path          string `json:"path"`
	Encoding      string `json:"encoding"`
	MimeType      string `json:"mime_type"`
	Size          int64  `json:"size"`
	Truncated     bool   `json:"truncated"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type FileRaw struct {
	Path        string
	Size        int64
	Truncated   bool
	Content     []byte
	ContentType string
}

type FileWrite struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type PathDelete struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type FileLines struct {
	Path          string   `json:"path"`
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	TotalLines    int      `json:"total_lines"`
	Lines         []string `json:"lines"`
	FileTruncated bool     `json:"file_truncated"`
	Size          int64    `json:"size"`
}

type FileLinesPatch struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	Size       int64  `json:"size"`
}

type GrepMatch struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
}

type GrepResponse struct {
	Object        string      `json:"object"`
	Matches       []GrepMatch `json:"matches"`
	FilesScanned  int         `json:"files_scanned"`
	ScanTruncated bool        `json:"scan_truncated"`
}

type SummaryPreview struct {
	Path             string `json:"path"`
	Size             int64  `json:"size"`
	Preview          string `json:"preview"`
	PreviewTruncated bool   `json:"preview_truncated,omitempty"`
}

type Summary struct {
	SummaryPath     string           `json:"summary_path"`
	FileCount       int              `json:"file_count"`
	TotalBytes      int64            `json:"total_bytes"`
	TopPathsBySize  []string         `json:"top_paths_by_size"`
	TextPreviews    []SummaryPreview `json:"text_previews"`
	GeneratedAtUnix int64            `json:"generated_at_unix"`
	ScanTruncated   bool             `json:"scan_truncated"`
}

type PathSensitivityInfo struct {
	Path        string          `json:"path"`
	Sensitivity PathSensitivity `json:"sensitivity"`
	Reason      string          `json:"reason,omitempty"`
}
