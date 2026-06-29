package agentapi

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type SkillsService struct{ http *httpClient }

type Skill struct {
	Object              string         `json:"object"`
	SkillID             string         `json:"skill_id"`
	TenantID            string         `json:"tenant_id,omitempty"`
	Name                string         `json:"name"`
	Description         string         `json:"description,omitempty"`
	SourceType          string         `json:"source_type,omitempty"`
	MainDigest          string         `json:"main_digest,omitempty"`
	DevDigest           string         `json:"dev_digest,omitempty"`
	HasDev              bool           `json:"has_dev,omitempty"`
	DevUpdatedAt        int64          `json:"dev_updated_at,omitempty"`
	DevSourceResponseID string         `json:"dev_source_response_id,omitempty"`
	CreatedByUserID     string         `json:"created_by_user_id,omitempty"`
	UpdatedByUserID     string         `json:"updated_by_user_id,omitempty"`
	CreatedAt           int64          `json:"created_at,omitempty"`
	UpdatedAt           int64          `json:"updated_at,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	Archived            bool           `json:"archived,omitempty"`
}

type SkillSummary struct {
	Object      string         `json:"object"`
	SkillID     string         `json:"skill_id"`
	SkillRef    string         `json:"skill_ref,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	SourceType  string         `json:"source_type,omitempty"`
	Branch      string         `json:"branch,omitempty"`
	Digest      string         `json:"digest,omitempty"`
	ArtifactURI string         `json:"artifact_uri,omitempty"`
	HasDev      bool           `json:"has_dev,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type FocusedSkill struct {
	SkillSummary
	MountHint         string             `json:"mount_hint,omitempty"`
	Manifest          string             `json:"manifest,omitempty"`
	ManifestTruncated bool               `json:"manifest_truncated,omitempty"`
	Entries           []SkillFileEntry   `json:"entries,omitempty"`
	Files             []SkillFocusedFile `json:"files,omitempty"`
}

type SkillFocusedFile struct {
	Path      string               `json:"path"`
	Content   string               `json:"content,omitempty"`
	Truncated bool                 `json:"truncated,omitempty"`
	Size      int64                `json:"size,omitempty"`
	Branch    string               `json:"branch,omitempty"`
	Error     *SkillOperationError `json:"error,omitempty"`
}

type SkillFocusItem struct {
	SkillID         string                `json:"skill_id,omitempty"`
	Branch          string                `json:"branch,omitempty"`
	Paths           []string              `json:"paths,omitempty"`
	IncludeManifest *bool                 `json:"include_manifest,omitempty"`
	LocalSkill      *LocalSkillDescriptor `json:"local_skill,omitempty"`
}

type SkillOperationError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type SkillFocusResultItem struct {
	OK           bool                 `json:"ok"`
	SkillRef     string               `json:"skill_ref,omitempty"`
	SkillID      string               `json:"skill_id,omitempty"`
	LocalSkillID string               `json:"local_skill_id,omitempty"`
	Branch       string               `json:"branch,omitempty"`
	Skill        *FocusedSkill        `json:"skill,omitempty"`
	Error        *SkillOperationError `json:"error,omitempty"`
}

type SkillFocusResponse struct {
	Object string                 `json:"object"`
	Data   []SkillFocusResultItem `json:"data"`
}

type SkillFileEntry struct {
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt int64  `json:"modified_at,omitempty"`
}

type ListSkillsParams struct {
	IncludeArchived bool
	Limit           int
	PageToken       string
	UserID          string
}

type ListSkillsResponse struct {
	Object        string  `json:"object"`
	Data          []Skill `json:"data"`
	NextPageToken string  `json:"next_page_token,omitempty"`
}

type ListSkillSummariesResponse struct {
	Object        string         `json:"object"`
	Data          []SkillSummary `json:"data"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

type DiscoverSkillsParams struct {
	Query              string                 `json:"query,omitempty"`
	Branch             string                 `json:"branch,omitempty"`
	IncludeDev         bool                   `json:"include_dev,omitempty"`
	Limit              int                    `json:"limit,omitempty"`
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	TenantSearch       bool                   `json:"tenant_search,omitempty"`
	LocalSkills        []LocalSkillDescriptor `json:"local_skills,omitempty"`
}

type FocusSkillParams struct {
	Skills           []SkillFocusItem `json:"skills"`
	FallbackToMain   *bool            `json:"fallback_to_main,omitempty"`
	MaxManifestChars int              `json:"max_manifest_chars,omitempty"`
	MaxFileChars     int              `json:"max_file_chars,omitempty"`
}

type SkillFileMutation struct {
	Path          string `json:"path"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type CreateSkillParams struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UpdateSkillParams struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreateSkillDevParams struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Metadata    map[string]any      `json:"metadata,omitempty"`
	Files       []SkillFileMutation `json:"files,omitempty"`
}

type CreateSkillDevResponse struct {
	Object       string           `json:"object"`
	Skill        Skill            `json:"skill"`
	Branch       string           `json:"branch"`
	Files        []SkillFileWrite `json:"files"`
	FocusedSkill *FocusedSkill    `json:"focused_skill,omitempty"`
}

type SkillFileWrite struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type SkillFileUpdateMutation struct {
	SkillID       string `json:"skill_id"`
	Path          string `json:"path"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type UpdateSkillFilePrimitiveParams struct {
	Updates []SkillFileUpdateMutation `json:"updates"`
}

type SkillUpdateResultItem struct {
	OK      bool                 `json:"ok"`
	SkillID string               `json:"skill_id,omitempty"`
	Path    string               `json:"path,omitempty"`
	Size    int64                `json:"size,omitempty"`
	Skill   *SkillSummary        `json:"skill,omitempty"`
	Error   *SkillOperationError `json:"error,omitempty"`
}

type UpdateSkillFilePrimitiveResponse struct {
	Object string                  `json:"object"`
	Data   []SkillUpdateResultItem `json:"data"`
}

type ListSkillFilesParams struct {
	Path           string
	Branch         string
	FallbackToMain *bool
	Limit          int
	PageToken      string
}

type ReadSkillFileParams struct {
	Branch         string
	FallbackToMain *bool
	MaxBytes       int
}

type ListSkillFilesResponse struct {
	Object        string           `json:"object"`
	Entries       []SkillFileEntry `json:"entries"`
	NextPageToken string           `json:"next_page_token,omitempty"`
}

type SkillFile struct {
	Object    string `json:"object"`
	Path      string `json:"path,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Content   string `json:"content,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
}

type SkillArchiveParams struct {
	Path           string
	Branch         string
	FallbackToMain *bool
}

type SkillArchive struct {
	Path        string
	Content     []byte
	ContentType string
}

type ImportSkillArchiveParams struct {
	Path             string
	Branch           string
	Replace          bool
	StripTopLevelDir *bool
}

type SkillImportResponse struct {
	Object    string `json:"object"`
	Branch    string `json:"branch"`
	FileCount int    `json:"file_count"`
	ByteCount int64  `json:"byte_count"`
	Skill     Skill  `json:"skill"`
}

type SkillBranchDiffParams struct {
	Path             string
	MaxFileChars     int
	IncludeUnchanged bool
}

type SkillBranchDiffFile struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	BaseSize    int64  `json:"base_size,omitempty"`
	CompareSize int64  `json:"compare_size,omitempty"`
	Text        bool   `json:"text,omitempty"`
	Binary      bool   `json:"binary,omitempty"`
	TooLarge    bool   `json:"too_large,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
	Diff        string `json:"diff,omitempty"`
}

type SkillBranchDiff struct {
	Object        string                `json:"object"`
	SkillID       string                `json:"skill_id"`
	BaseBranch    string                `json:"base_branch"`
	CompareBranch string                `json:"compare_branch"`
	Path          string                `json:"path,omitempty"`
	Summary       map[string]int        `json:"summary"`
	Files         []SkillBranchDiffFile `json:"files"`
}

type SkillDirectoryPullResult struct {
	Path      string `json:"path"`
	FileCount int    `json:"file_count"`
	ByteCount int64  `json:"byte_count"`
}

func (s *SkillsService) List(ctx context.Context, params ListSkillsParams, opts ...RequestOption) (*ListSkillsResponse, error) {
	var out ListSkillsResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/skills"+buildQuery(map[string]any{
		"include_archived": params.IncludeArchived,
		"limit":            params.Limit,
		"page_token":       params.PageToken,
		"user_id":          params.UserID,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) Create(ctx context.Context, params CreateSkillParams, opts ...RequestOption) (*Skill, error) {
	var out Skill
	err := s.http.requestJSON(ctx, "POST", "/v1/skills", params, &out, opts...)
	return &out, err
}

func (s *SkillsService) Discover(ctx context.Context, params DiscoverSkillsParams, opts ...RequestOption) (*ListSkillSummariesResponse, error) {
	var out ListSkillSummariesResponse
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/discover", params, &out, opts...)
	return &out, err
}

func (s *SkillsService) Focus(ctx context.Context, params FocusSkillParams, opts ...RequestOption) (*SkillFocusResponse, error) {
	var out SkillFocusResponse
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/focus", params, &out, opts...)
	return &out, err
}

func (s *SkillsService) CreateDev(ctx context.Context, params CreateSkillDevParams, opts ...RequestOption) (*CreateSkillDevResponse, error) {
	var out CreateSkillDevResponse
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/create_dev", params, &out, opts...)
	return &out, err
}

func (s *SkillsService) UpdateFile(ctx context.Context, params UpdateSkillFilePrimitiveParams, opts ...RequestOption) (*UpdateSkillFilePrimitiveResponse, error) {
	var out UpdateSkillFilePrimitiveResponse
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/update_file", params, &out, opts...)
	return &out, err
}

func (s *SkillsService) Retrieve(ctx context.Context, skillID string, opts ...RequestOption) (*Skill, error) {
	var out Skill
	err := s.http.requestJSON(ctx, "GET", "/v1/skills/"+url.PathEscape(skillID), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) Update(ctx context.Context, skillID string, params UpdateSkillParams, opts ...RequestOption) (*Skill, error) {
	body := map[string]any{}
	if params.Name != "" {
		body["name"] = params.Name
	}
	if params.Description != "" {
		body["description"] = params.Description
	}
	if params.Metadata != nil {
		body["metadata"] = params.Metadata
	}
	var out Skill
	err := s.http.requestJSON(ctx, "PATCH", "/v1/skills/"+url.PathEscape(skillID), body, &out, opts...)
	return &out, err
}

func (s *SkillsService) Archive(ctx context.Context, skillID string, opts ...RequestOption) (*Skill, error) {
	var out Skill
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/"+url.PathEscape(skillID)+"/archive", map[string]any{}, &out, opts...)
	return &out, err
}

func (s *SkillsService) Delete(ctx context.Context, skillID string, opts ...RequestOption) (map[string]bool, error) {
	var out map[string]bool
	err := s.http.requestJSON(ctx, "DELETE", "/v1/skills/"+url.PathEscape(skillID), nil, &out, opts...)
	return out, err
}

func (s *SkillsService) AcceptDev(ctx context.Context, skillID, strategy string, opts ...RequestOption) (*Skill, error) {
	var out Skill
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/"+url.PathEscape(skillID)+"/accept_dev"+buildQuery(map[string]any{"strategy": strategy}), map[string]any{}, &out, opts...)
	return &out, err
}

func (s *SkillsService) DiscardDev(ctx context.Context, skillID string, opts ...RequestOption) (*Skill, error) {
	var out Skill
	err := s.http.requestJSON(ctx, "POST", "/v1/skills/"+url.PathEscape(skillID)+"/discard_dev", map[string]any{}, &out, opts...)
	return &out, err
}

func (s *SkillsService) ListFiles(ctx context.Context, skillID string, params ListSkillFilesParams, opts ...RequestOption) (*ListSkillFilesResponse, error) {
	var out ListSkillFilesResponse
	err := s.http.requestJSON(ctx, "GET", "/v1/skills/"+url.PathEscape(skillID)+"/files"+buildQuery(map[string]any{
		"path":             params.Path,
		"branch":           params.Branch,
		"fallback_to_main": boolPtrQuery(params.FallbackToMain),
		"limit":            params.Limit,
		"page_token":       params.PageToken,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) ReadFile(ctx context.Context, skillID, path string, params ReadSkillFileParams, opts ...RequestOption) (*SkillFile, error) {
	var out SkillFile
	err := s.http.requestJSON(ctx, "GET", "/v1/skills/"+url.PathEscape(skillID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{
		"branch":           params.Branch,
		"fallback_to_main": boolPtrQuery(params.FallbackToMain),
		"max_bytes":        params.MaxBytes,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) WriteFile(ctx context.Context, skillID, path string, content []byte, branch string, opts ...RequestOption) (*SkillFile, error) {
	var out SkillFile
	err := s.http.requestRawJSON(ctx, "PUT", "/v1/skills/"+url.PathEscape(skillID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{"branch": branch}), bytes.NewReader(content), &out, opts...)
	return &out, err
}

func (s *SkillsService) WriteFileReader(ctx context.Context, skillID, path string, content io.Reader, branch string, opts ...RequestOption) (*SkillFile, error) {
	var out SkillFile
	err := s.http.requestRawJSON(ctx, "PUT", "/v1/skills/"+url.PathEscape(skillID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{"branch": branch}), content, &out, opts...)
	return &out, err
}

func (s *SkillsService) DeleteFile(ctx context.Context, skillID, path, branch string, opts ...RequestOption) (*SkillFile, error) {
	var out SkillFile
	err := s.http.requestJSON(ctx, "DELETE", "/v1/skills/"+url.PathEscape(skillID)+"/files/"+pathEscapePath(path)+buildQuery(map[string]any{"branch": branch}), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) ExportArchive(ctx context.Context, skillID string, params SkillArchiveParams, opts ...RequestOption) (*SkillArchive, error) {
	archivePath := normalizeArchivePath(params.Path)
	data, h, err := s.http.requestBinary(ctx, "GET", "/v1/skills/"+url.PathEscape(skillID)+"/export"+buildQuery(map[string]any{
		"path":             archivePath,
		"branch":           params.Branch,
		"fallback_to_main": boolPtrQuery(params.FallbackToMain),
	}), opts...)
	if err != nil {
		return nil, err
	}
	return &SkillArchive{Path: archivePath, Content: data, ContentType: h.Get("Content-Type")}, nil
}

func (s *SkillsService) ImportArchive(ctx context.Context, skillID string, archive []byte, params ImportSkillArchiveParams, opts ...RequestOption) (*SkillImportResponse, error) {
	var out SkillImportResponse
	err := s.http.requestRawJSON(ctx, "POST", "/v1/skills/"+url.PathEscape(skillID)+"/import"+buildQuery(map[string]any{
		"path":                normalizeArchivePath(params.Path),
		"branch":              params.Branch,
		"replace":             params.Replace,
		"strip_top_level_dir": boolPtrQuery(params.StripTopLevelDir),
	}), bytes.NewReader(archive), &out, opts...)
	return &out, err
}

func (s *SkillsService) Diff(ctx context.Context, skillID string, params SkillBranchDiffParams, opts ...RequestOption) (*SkillBranchDiff, error) {
	var out SkillBranchDiff
	err := s.http.requestJSON(ctx, "GET", "/v1/skills/"+url.PathEscape(skillID)+"/diff"+buildQuery(map[string]any{
		"path":              normalizeArchivePath(params.Path),
		"max_file_chars":    params.MaxFileChars,
		"include_unchanged": params.IncludeUnchanged,
	}), nil, &out, opts...)
	return &out, err
}

func (s *SkillsService) PushDirectory(ctx context.Context, skillID, rootDir string, params ImportSkillArchiveParams, opts ...RequestOption) (*SkillImportResponse, error) {
	archive, err := ZipDirectory(rootDir)
	if err != nil {
		return nil, err
	}
	return s.ImportArchive(ctx, skillID, archive, params, opts...)
}

func (s *SkillsService) PullDirectory(ctx context.Context, skillID, targetDir string, params SkillArchiveParams, replace bool, opts ...RequestOption) (*SkillDirectoryPullResult, error) {
	archive, err := s.ExportArchive(ctx, skillID, params, opts...)
	if err != nil {
		return nil, err
	}
	return ExtractZipToDirectory(archive.Content, targetDir, replace)
}

func ZipDirectory(rootDir string) ([]byte, error) {
	root, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && ignoredArchiveDir(d.Name()) && path != root {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ExtractZipToDirectory(archive []byte, targetDir string, replace bool) (*SkillDirectoryPullResult, error) {
	root, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, err
	}
	if replace {
		if err := os.RemoveAll(root); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	var files int
	var bytesWritten int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimLeft(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		if name == "" || strings.HasPrefix(name, "__MACOSX/") || strings.Contains(name, "../") {
			continue
		}
		dest := filepath.Join(root, filepath.FromSlash(name))
		rel, err := filepath.Rel(root, dest)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("archive entry escapes target directory: %s", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return nil, err
		}
		n, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files++
		bytesWritten += n
	}
	return &SkillDirectoryPullResult{Path: root, FileCount: files, ByteCount: bytesWritten}, nil
}

func boolPtrQuery(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func ignoredArchiveDir(name string) bool {
	return name == ".git" || name == "__pycache__" || name == "node_modules"
}
