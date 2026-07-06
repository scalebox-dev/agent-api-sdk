package local

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

type WorkdirManager struct{}

type WorkdirOptions struct {
	Name             string
	Metadata         map[string]any
	Trusted          bool
	Ignore           []IgnoreRule
	DisableGitignore bool
	MaxFileBytes     int
}

type Workdir struct {
	Root         string
	Name         string
	Metadata     map[string]any
	Trusted      bool
	Files        *FileStore
	Ignore       []IgnoreRule
	Gitignore    bool
	MaxFileBytes int
}

type SnapshotParams struct {
	Path            string
	OmitHash        bool
	MaxBytesPerFile int
}

type SnapshotFile struct {
	Path           string `json:"path"`
	Size           int64  `json:"size"`
	ModifiedAtUnix int64  `json:"modified_at_unix,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
}

type Snapshot struct {
	Root            string         `json:"root"`
	Name            string         `json:"name"`
	GeneratedAtUnix int64          `json:"generated_at_unix"`
	Files           []SnapshotFile `json:"files"`
}

type Diff struct {
	Added     []SnapshotFile `json:"added"`
	Modified  []DiffModified `json:"modified"`
	Deleted   []SnapshotFile `json:"deleted"`
	Unchanged []SnapshotFile `json:"unchanged"`
}

type DiffModified struct {
	Before SnapshotFile `json:"before"`
	After  SnapshotFile `json:"after"`
}

type LinePatchPreview struct {
	Path       string   `json:"path"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	TotalLines int      `json:"total_lines"`
	Before     []string `json:"before"`
	After      []string `json:"after"`
}

type LineEdit struct {
	Path           string
	StartLine      int
	EndLine        int
	Replacement    string
	ExpectedSHA256 string
}

type EditPlan struct {
	Edits    []LineEdit         `json:"edits"`
	Previews []LinePatchPreview `json:"previews"`
}

type EditResult struct {
	Applied      []FileLinesPatch `json:"applied"`
	ChangedFiles []string         `json:"changed_files"`
	EditCount    int              `json:"edit_count"`
}

type EditBackup struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (m *WorkdirManager) Open(root string, opts WorkdirOptions) (*Workdir, error) {
	return NewWorkdir(root, opts)
}

func NewWorkdir(root string, opts WorkdirOptions) (*Workdir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	name := opts.Name
	if name == "" {
		name = filepath.Base(abs)
	}
	if name == "" || name == "." {
		name = "workdir"
	}
	files, err := NewFileStore(abs, "workdir")
	if err != nil {
		return nil, err
	}
	ignore := append([]IgnoreRule{}, DefaultWorkdirIgnoreRules()...)
	ignore = append(ignore, opts.Ignore...)
	maxBytes := opts.MaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	return &Workdir{Root: abs, Name: name, Metadata: opts.Metadata, Trusted: opts.Trusted, Files: files, Ignore: ignore, Gitignore: !opts.DisableGitignore, MaxFileBytes: maxBytes}, nil
}

func (w *Workdir) Ensure() error {
	if err := w.Files.Ensure(); err != nil {
		return err
	}
	if w.Gitignore {
		_, err := w.LoadIgnoreFiles(".gitignore")
		return err
	}
	return nil
}

func (w *Workdir) LoadIgnoreFiles(files ...string) ([]IgnoreRule, error) {
	if len(files) == 0 {
		files = []string{".gitignore"}
	}
	var loaded []IgnoreRule
	for _, file := range files {
		text, err := w.Files.ReadText(file)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		loaded = append(loaded, ParseIgnoreFile(text)...)
	}
	w.Ignore = append(w.Ignore, loaded...)
	return loaded, nil
}

func (w *Workdir) ResolvePath(rel string) (string, error) {
	if err := w.assertAllowed(rel); err != nil {
		return "", err
	}
	return w.Files.ResolvePath(rel)
}

func (w *Workdir) List(rel string, opts ListOptions) ([]FileStat, error) {
	opts.Ignore = w.mergeIgnore(opts.Ignore)
	return w.Files.List(rel, opts)
}

func (w *Workdir) ListWithWarnings(rel string, opts ListOptions) ([]FileStat, []ScanWarning, error) {
	opts.Ignore = w.mergeIgnore(opts.Ignore)
	return w.Files.ListWithWarnings(rel, opts)
}

func (w *Workdir) ListEntries(rel string, opts ListOptions) (*EntryList, error) {
	opts.Ignore = w.mergeIgnore(opts.Ignore)
	return w.Files.ListEntries(rel, opts)
}

func (w *Workdir) SearchEntries(query, rel string, limit int) (*EntryList, error) {
	stats, err := w.List(rel, ListOptions{Recursive: true, IncludeDirectories: true})
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	var entries []Entry
	for _, item := range stats {
		if containsFold(item.Path, query) {
			entries = append(entries, entryFromStat(item))
			if len(entries) >= limit {
				break
			}
		}
	}
	return &EntryList{Object: "list", Entries: entries}, nil
}

func (w *Workdir) ReadFile(rel string, params ReadFileParams) (*FileDeliver, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	if params.MaxBytes <= 0 {
		params.MaxBytes = w.MaxFileBytes
	}
	return w.Files.ReadFile(rel, params)
}

func (w *Workdir) ReadFileRaw(rel string, params ReadFileParams) (*FileRaw, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	if params.MaxBytes <= 0 {
		params.MaxBytes = w.MaxFileBytes
	}
	return w.Files.ReadFileRaw(rel, params)
}

func (w *Workdir) WriteText(rel, content string) (string, error) {
	if err := w.assertAllowed(rel); err != nil {
		return "", err
	}
	return w.Files.WriteText(rel, content)
}

func (w *Workdir) ReadText(rel string) (string, error) {
	if err := w.assertAllowed(rel); err != nil {
		return "", err
	}
	return w.Files.ReadText(rel)
}

func (w *Workdir) WriteFile(rel string, content []byte) (*FileWrite, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	return w.Files.WriteFile(rel, content)
}

func (w *Workdir) DeletePath(rel string) (*PathDelete, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	return w.Files.DeletePath(rel)
}

func (w *Workdir) CreateDirectory(rel string) (map[string]string, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	return w.Files.CreateDirectory(rel)
}

func (w *Workdir) ReadLines(rel string, params ReadLinesParams) (*FileLines, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	if params.MaxBytes <= 0 {
		params.MaxBytes = w.MaxFileBytes
	}
	return w.Files.ReadLines(rel, params)
}

func (w *Workdir) PreviewPatchLines(rel string, params PatchLinesParams) (*LinePatchPreview, error) {
	lines, err := w.ReadLines(rel, ReadLinesParams{StartLine: params.StartLine, EndLine: params.EndLine, MaxBytes: firstPositiveInt(params.MaxBytes, w.MaxFileBytes)})
	if err != nil {
		return nil, err
	}
	file, err := w.ReadFileRaw(rel, ReadFileParams{MaxBytes: firstPositiveInt(params.MaxBytes, w.MaxFileBytes)})
	if err != nil {
		return nil, err
	}
	if file.Truncated {
		return nil, fileTooLargeError(rel)
	}
	patched, _, total, err := patchLineRange(string(file.Content), params.StartLine, params.EndLine, params.Replacement)
	if err != nil {
		return nil, err
	}
	_ = patched
	after := []string{}
	if params.Replacement != "" {
		after = splitLines(params.Replacement)
	}
	return &LinePatchPreview{Path: lines.Path, StartLine: lines.StartLine, EndLine: lines.EndLine, TotalLines: total, Before: lines.Lines, After: after}, nil
}

func (w *Workdir) PatchLines(rel string, params PatchLinesParams) (*FileLinesPatch, error) {
	if err := w.assertAllowed(rel); err != nil {
		return nil, err
	}
	if params.MaxBytes <= 0 {
		params.MaxBytes = w.MaxFileBytes
	}
	return w.Files.PatchLines(rel, params)
}

func (w *Workdir) PreviewEdits(edits []LineEdit) (*EditPlan, error) {
	previews := make([]LinePatchPreview, 0, len(edits))
	for _, edit := range edits {
		if err := w.assertExpectedHash(edit.Path, edit.ExpectedSHA256); err != nil {
			return nil, err
		}
		preview, err := w.PreviewPatchLines(edit.Path, PatchLinesParams{StartLine: edit.StartLine, EndLine: edit.EndLine, Replacement: edit.Replacement})
		if err != nil {
			return nil, err
		}
		previews = append(previews, *preview)
	}
	return &EditPlan{Edits: append([]LineEdit(nil), edits...), Previews: previews}, nil
}

func (w *Workdir) ApplyEdits(edits []LineEdit) (*EditResult, error) {
	var backups []EditBackup
	var applied []FileLinesPatch
	for _, edit := range edits {
		if err := w.assertExpectedHash(edit.Path, edit.ExpectedSHA256); err != nil {
			restoreBackups(w, backups)
			return nil, err
		}
		content, err := w.ReadText(edit.Path)
		if err != nil {
			restoreBackups(w, backups)
			return nil, err
		}
		backups = append(backups, EditBackup{Path: edit.Path, Content: content})
		patch, err := w.PatchLines(edit.Path, PatchLinesParams{StartLine: edit.StartLine, EndLine: edit.EndLine, Replacement: edit.Replacement})
		if err != nil {
			restoreBackups(w, backups)
			return nil, err
		}
		applied = append(applied, *patch)
	}
	return &EditResult{Applied: applied, ChangedFiles: uniquePatchPaths(applied), EditCount: len(applied)}, nil
}

func (w *Workdir) ClassifyPath(rel string) PathSensitivityInfo {
	return ClassifyPathSensitivity(rel)
}

func (w *Workdir) Grep(params GrepParams) (*GrepResponse, error) {
	params.Ignore = w.mergeIgnore(params.Ignore)
	return w.Files.Grep(params)
}

func (w *Workdir) Summarize(params SummaryParams) (*Summary, error) {
	params.Ignore = w.mergeIgnore(params.Ignore)
	return w.Files.Summarize(params)
}

func (w *Workdir) Snapshot(params SnapshotParams) (*Snapshot, error) {
	path := firstNonEmpty(params.Path, ".")
	maxBytes := firstPositiveInt(params.MaxBytesPerFile, w.MaxFileBytes)
	hashFiles := !params.OmitHash
	stats, err := w.List(path, ListOptions{Recursive: true})
	if err != nil {
		return nil, err
	}
	out := &Snapshot{Root: w.Root, Name: w.Name, GeneratedAtUnix: time.Now().Unix()}
	for _, item := range stats {
		if item.Type != FileTypeFile {
			continue
		}
		file := SnapshotFile{Path: item.Path, Size: item.Size, ModifiedAtUnix: item.ModifiedAt.Unix()}
		if hashFiles && item.Size <= int64(maxBytes) {
			raw, err := os.ReadFile(item.FullPath)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(raw)
			file.SHA256 = hex.EncodeToString(sum[:])
		}
		out.Files = append(out.Files, file)
	}
	return out, nil
}

func (w *Workdir) Diff(before, after *Snapshot) Diff {
	beforeByPath := map[string]SnapshotFile{}
	afterByPath := map[string]SnapshotFile{}
	for _, file := range before.Files {
		beforeByPath[file.Path] = file
	}
	for _, file := range after.Files {
		afterByPath[file.Path] = file
	}
	var diff Diff
	for _, file := range after.Files {
		old, ok := beforeByPath[file.Path]
		if !ok {
			diff.Added = append(diff.Added, file)
		} else if snapshotFileChanged(old, file) {
			diff.Modified = append(diff.Modified, DiffModified{Before: old, After: file})
		} else {
			diff.Unchanged = append(diff.Unchanged, file)
		}
	}
	for _, file := range before.Files {
		if _, ok := afterByPath[file.Path]; !ok {
			diff.Deleted = append(diff.Deleted, file)
		}
	}
	return diff
}

func (w *Workdir) mergeIgnore(extra []IgnoreRule) []IgnoreRule {
	out := append([]IgnoreRule{}, w.Ignore...)
	out = append(out, extra...)
	return out
}

func (w *Workdir) assertAllowed(rel string) error {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		return err
	}
	if clean != "." && Ignored(clean, w.Ignore) {
		return ignoredPathError(clean)
	}
	return nil
}

func (w *Workdir) assertExpectedHash(rel, expected string) error {
	if expected == "" {
		return nil
	}
	full, err := w.Files.ResolvePath(rel)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != expected {
		return editConflictError(rel)
	}
	return nil
}

func restoreBackups(w *Workdir, backups []EditBackup) {
	for i := len(backups) - 1; i >= 0; i-- {
		_, _ = w.WriteText(backups[i].Path, backups[i].Content)
	}
}

func uniquePatchPaths(patches []FileLinesPatch) []string {
	seen := map[string]bool{}
	var paths []string
	for _, patch := range patches {
		if seen[patch.Path] {
			continue
		}
		seen[patch.Path] = true
		paths = append(paths, patch.Path)
	}
	return paths
}
