package agentapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type VolumesService struct{ http *httpClient }

type VolumeInfo struct {
	VolumeID              string `json:"volume_id"`
	TenantID              string `json:"tenant_id,omitempty"`
	Name                  string `json:"name,omitempty"`
	OSSPrefix             string `json:"oss_prefix,omitempty"`
	BytesUsed             int64  `json:"bytes_used,omitempty"`
	ObjectCount           int64  `json:"object_count,omitempty"`
	UsageReconciledAtUnix int64  `json:"usage_reconciled_at_unix,omitempty"`
	CreatedAtUnix         int64  `json:"created_at_unix,omitempty"`
	UpdatedAtUnix         int64  `json:"updated_at_unix,omitempty"`
}

type VolumeEntry struct {
	Path           string `json:"path"`
	IsDir          bool   `json:"is_dir"`
	Size           int64  `json:"size"`
	ModifiedAtUnix int64  `json:"modified_at_unix,omitempty"`
}

type ListVolumesResponse struct {
	Object        string       `json:"object"`
	Data          []VolumeInfo `json:"data"`
	NextPageToken string       `json:"next_page_token,omitempty"`
}

type ListVolumeEntriesResponse struct {
	Object        string        `json:"object"`
	Entries       []VolumeEntry `json:"entries"`
	NextPageToken string        `json:"next_page_token,omitempty"`
}

type VolumeFileDeliver struct {
	Path               string   `json:"path"`
	Encoding           string   `json:"encoding"`
	MimeType           string   `json:"mime_type"`
	Size               int64    `json:"size"`
	Truncated          bool     `json:"truncated"`
	Content            string   `json:"content,omitempty"`
	ContentBase64      string   `json:"content_base64,omitempty"`
	ImageURL           string   `json:"image_url,omitempty"`
	ExpiresAtUnix      int64    `json:"expires_at_unix,omitempty"`
	ExtractionWarnings []string `json:"extraction_warnings,omitempty"`
}

type VolumeFileRaw struct {
	Path        string
	Size        int64
	Truncated   bool
	Content     []byte
	ContentType string
}

type VolumeFileWrite struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type VolumePathDelete struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type VolumeArchive struct {
	Path        string
	Content     []byte
	ContentType string
}

type VolumeSummary struct {
	SummaryPath     string                 `json:"summary_path"`
	FileCount       int                    `json:"file_count"`
	TotalBytes      int64                  `json:"total_bytes"`
	TopPathsBySize  []string               `json:"top_paths_by_size"`
	TextPreviews    []VolumeSummaryPreview `json:"text_previews"`
	GeneratedAtUnix int64                  `json:"generated_at_unix"`
}

type VolumeSummaryPreview struct {
	Path             string `json:"path"`
	Size             int64  `json:"size"`
	Preview          string `json:"preview"`
	PreviewTruncated bool   `json:"preview_truncated,omitempty"`
}

type VolumeFileLines struct {
	Path          string   `json:"path"`
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	TotalLines    int      `json:"total_lines"`
	Lines         []string `json:"lines"`
	FileTruncated bool     `json:"file_truncated"`
	Size          int64    `json:"size"`
}

type VolumeFileLinesPatch struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	Size       int64  `json:"size"`
}

type VolumeGrepResponse struct {
	Object        string            `json:"object"`
	Matches       []VolumeGrepMatch `json:"matches"`
	NextPageToken string            `json:"next_page_token,omitempty"`
	FilesScanned  int               `json:"files_scanned"`
	ScanTruncated bool              `json:"scan_truncated"`
}

type VolumeGrepMatch struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
}

type VolumeEntriesParams struct {
	Path      string
	Query     string
	Limit     int
	PageToken string
}

type ReadFileParams struct {
	MaxBytes int
}

type ReadLinesParams struct {
	StartLine int
	EndLine   int
	MaxBytes  int
}

type PatchLinesParams struct {
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

func (s *VolumesService) List(ctx context.Context, params ListParams, opts ...RequestOption) (*ListVolumesResponse, error) {
	var out ListVolumesResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes"+buildQuery(map[string]any{"limit": params.Limit, "page_token": params.PageToken}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) Create(ctx context.Context, name string, opts ...RequestOption) (*VolumeInfo, error) {
	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}
	var out VolumeInfo
	err := s.http.requestJSON(ctx, "POST", "/v1/volumes", body, &out, opts...)
	return &out, err
}

func (s *VolumesService) Retrieve(ctx context.Context, volumeID string, opts ...RequestOption) (*VolumeInfo, error) {
	var out VolumeInfo
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) Update(ctx context.Context, volumeID, name string, opts ...RequestOption) (*VolumeInfo, error) {
	var out VolumeInfo
	err := s.http.requestJSON(ctx, "PATCH", "/v1/volumes/"+url.PathEscape(volumeID), map[string]string{"name": name}, &out, opts...)
	return &out, err
}

func (s *VolumesService) Delete(ctx context.Context, volumeID string, opts ...RequestOption) error {
	return s.http.requestJSON(ctx, "DELETE", "/v1/volumes/"+url.PathEscape(volumeID), nil, nil, opts...)
}

func (s *VolumesService) ReconcileUsage(ctx context.Context, volumeID string, opts ...RequestOption) (*VolumeInfo, error) {
	var out VolumeInfo
	err := s.http.requestJSON(ctx, "POST", "/v1/volumes/"+url.PathEscape(volumeID)+"/usage/reconcile", nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) ListEntries(ctx context.Context, volumeID string, params VolumeEntriesParams, opts ...RequestOption) (*ListVolumeEntriesResponse, error) {
	var out ListVolumeEntriesResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/entries"+buildQuery(map[string]any{
		"path": params.Path, "limit": params.Limit, "page_token": params.PageToken,
	}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) SearchEntries(ctx context.Context, volumeID string, params VolumeEntriesParams, opts ...RequestOption) (*ListVolumeEntriesResponse, error) {
	var out ListVolumeEntriesResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/search"+buildQuery(map[string]any{
		"query": params.Query, "path": params.Path, "limit": params.Limit, "page_token": params.PageToken,
	}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) ReadFile(ctx context.Context, volumeID, path string, params ReadFileParams, opts ...RequestOption) (*VolumeFileDeliver, error) {
	var out VolumeFileDeliver
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{"max_bytes": params.MaxBytes}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) ReadFileRaw(ctx context.Context, volumeID, path string, params ReadFileParams, opts ...RequestOption) (*VolumeFileRaw, error) {
	data, h, err := s.http.requestBinary(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{"max_bytes": params.MaxBytes, "format": "raw"}), opts...)
	if err != nil {
		return nil, err
	}
	size, _ := strconv.ParseInt(h.Get("X-Volume-Size"), 10, 64)
	if size == 0 {
		size = int64(len(data))
	}
	return &VolumeFileRaw{Path: path, Size: size, Truncated: h.Get("X-Volume-Truncated") == "true", Content: data, ContentType: h.Get("Content-Type")}, nil
}

func (s *VolumesService) WriteFile(ctx context.Context, volumeID, path string, content []byte, opts ...RequestOption) (*VolumeFileWrite, error) {
	var out VolumeFileWrite
	err := s.http.requestRawJSON(ctx, "PUT", "/v1/volumes/"+url.PathEscape(volumeID)+"/files/"+pathEscapePath(path), bytes.NewReader(content), &out, opts...)
	return &out, err
}

func (s *VolumesService) WriteFileReader(ctx context.Context, volumeID, path string, content io.Reader, opts ...RequestOption) (*VolumeFileWrite, error) {
	var out VolumeFileWrite
	err := s.http.requestRawJSON(ctx, "PUT", "/v1/volumes/"+url.PathEscape(volumeID)+"/files/"+pathEscapePath(path), content, &out, opts...)
	return &out, err
}

func (s *VolumesService) DeletePath(ctx context.Context, volumeID, path string, opts ...RequestOption) (*VolumePathDelete, error) {
	var out VolumePathDelete
	err := s.http.requestJSON(ctx, "DELETE", "/v1/volumes/"+url.PathEscape(volumeID)+"/paths/"+pathEscapePath(path), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) CreateDirectory(ctx context.Context, volumeID, path string, opts ...RequestOption) (map[string]string, error) {
	var out map[string]string
	err := s.http.requestJSON(ctx, "POST", "/v1/volumes/"+url.PathEscape(volumeID)+"/directories", map[string]string{"path": path}, &out, opts...)
	return out, err
}

func (s *VolumesService) DownloadArchive(ctx context.Context, volumeID, path string, opts ...RequestOption) (*VolumeArchive, error) {
	archivePath := normalizeArchivePath(path)
	data, h, err := s.http.requestBinary(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/archive"+buildQuery(map[string]any{"path": archivePath}), opts...)
	if err != nil {
		return nil, err
	}
	return &VolumeArchive{Path: archivePath, Content: data, ContentType: h.Get("Content-Type")}, nil
}

func (s *VolumesService) Summarize(ctx context.Context, volumeID, path string, opts ...RequestOption) (*VolumeSummary, error) {
	var out VolumeSummary
	body := map[string]string{}
	if path != "" {
		body["path"] = path
	}
	err := s.http.requestJSON(ctx, "POST", "/v1/volumes/"+url.PathEscape(volumeID)+"/summarize", body, &out, opts...)
	return &out, err
}

func (s *VolumesService) Grep(ctx context.Context, volumeID string, params VolumeEntriesParams, opts ...RequestOption) (*VolumeGrepResponse, error) {
	var out VolumeGrepResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/grep"+buildQuery(map[string]any{
		"pattern": params.Query, "path": params.Path, "limit": params.Limit, "page_token": params.PageToken,
	}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) ReadLines(ctx context.Context, volumeID, path string, params ReadLinesParams, opts ...RequestOption) (*VolumeFileLines, error) {
	var out VolumeFileLines
	err := s.http.requestJSON(ctx, "GET", "/v1/volumes/"+url.PathEscape(volumeID)+"/file_lines/"+pathEscapePath(path)+buildQuery(map[string]any{
		"start_line": params.StartLine, "end_line": params.EndLine, "max_bytes": params.MaxBytes,
	}), nil, &out, opts...)
	return &out, err
}

func (s *VolumesService) PatchLines(ctx context.Context, volumeID, path string, params PatchLinesParams, opts ...RequestOption) (*VolumeFileLinesPatch, error) {
	var out VolumeFileLinesPatch
	err := s.http.requestJSON(ctx, "PATCH", "/v1/volumes/"+url.PathEscape(volumeID)+"/file_lines/"+pathEscapePath(path), params, &out, opts...)
	return &out, err
}

func _headerClone(h http.Header) http.Header { return h.Clone() }
