package local

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const maxScanWarnings = 20

type FileStore struct {
	Root  string
	Label string
}

type ListOptions struct {
	Recursive          bool
	IncludeDirectories bool
	MaxDepth           int
	Ignore             []IgnoreRule
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
	StartLine   int
	EndLine     int
	Replacement string
	MaxBytes    int
}

type GrepParams struct {
	Pattern         string
	Path            string
	Limit           int
	MaxFiles        int
	MaxBytesPerFile int
	MaxLineLength   int
	Ignore          []IgnoreRule
}

type SummaryParams struct {
	Path         string
	MaxFiles     int
	MaxPreviews  int
	PreviewBytes int
	TopPaths     int
	MaxDepth     int
	Ignore       []IgnoreRule
}

func NewFileStore(root string, label string) (*FileStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if label == "" {
		label = "local"
	}
	return &FileStore{Root: abs, Label: label}, nil
}

func (s *FileStore) Child(rel string, label string) (*FileStore, error) {
	full, _, err := ResolveInside(s.Root, rel)
	if err != nil {
		return nil, err
	}
	if label == "" {
		label = s.Label
	}
	return NewFileStore(full, label)
}

func (s *FileStore) Ensure() error {
	return os.MkdirAll(s.Root, 0o755)
}

func (s *FileStore) ResolvePath(rel string) (string, error) {
	full, _, err := ResolveInside(s.Root, rel)
	return full, err
}

func (s *FileStore) RelativePath(fullPath string) (string, error) {
	abs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	root, _ := filepath.Abs(s.Root)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", pathError("local path must stay inside the store root", fullPath)
	}
	return filepath.ToSlash(rel), nil
}

func (s *FileStore) Exists(rel string) bool {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

func (s *FileStore) Stat(rel string) (*FileStat, error) {
	full, portable, err := ResolveInside(s.Root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	return &FileStat{Path: firstNonEmpty(portable, "."), FullPath: full, Type: fileType(info), Size: info.Size(), ModifiedAt: info.ModTime()}, nil
}

func (s *FileStore) Mkdir(rel string) (string, error) {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return "", err
	}
	return full, os.MkdirAll(full, 0o755)
}

func (s *FileStore) List(rel string, opts ListOptions) ([]FileStat, error) {
	stats, _, err := s.listWithWarnings(rel, opts)
	return stats, err
}

func (s *FileStore) listWithWarnings(rel string, opts ListOptions) ([]FileStat, []ScanWarning, error) {
	base, _, err := ResolveInside(s.Root, rel)
	if err != nil {
		return nil, nil, err
	}
	maxDepth := opts.MaxDepth
	if !opts.Recursive {
		maxDepth = 1
	}
	var out []FileStat
	var warnings []ScanWarning
	err = s.walk(s.Root, base, base, maxDepth, opts, &out, &warnings)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, warnings, err
}

func (s *FileStore) ListEntries(rel string, opts ListOptions) (*EntryList, error) {
	opts.IncludeDirectories = true
	stats, warnings, err := s.listWithWarnings(rel, opts)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(stats))
	for _, item := range stats {
		entries = append(entries, entryFromStat(item))
	}
	return &EntryList{Object: "list", Entries: entries, ScanWarnings: warnings}, nil
}

func (s *FileStore) SearchEntries(query, rel string, limit int) (*EntryList, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 100
	}
	stats, err := s.List(rel, ListOptions{Recursive: true, IncludeDirectories: true})
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, item := range stats {
		if strings.Contains(strings.ToLower(item.Path), query) {
			entries = append(entries, entryFromStat(item))
			if len(entries) >= limit {
				break
			}
		}
	}
	return &EntryList{Object: "list", Entries: entries}, nil
}

func (s *FileStore) ReadText(rel string) (string, error) {
	raw, err := s.ReadBytes(rel)
	return string(raw), err
}

func (s *FileStore) ReadJSON(rel string, out any) error {
	raw, err := s.ReadBytes(rel)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (s *FileStore) ReadBytes(rel string) ([]byte, error) {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *FileStore) ReadFile(rel string, params ReadFileParams) (*FileDeliver, error) {
	raw, info, portable, err := s.readRaw(rel, params.MaxBytes)
	if err != nil {
		return nil, err
	}
	truncated := params.MaxBytes > 0 && int64(len(raw)) < info.Size()
	deliver := &FileDeliver{Path: portable, MimeType: mimeType(portable), Size: info.Size(), Truncated: truncated}
	if looksBinary(raw) || !utf8.Valid(raw) {
		deliver.Encoding = "base64"
		deliver.ContentBase64 = base64.StdEncoding.EncodeToString(raw)
	} else {
		deliver.Encoding = "text"
		deliver.Content = string(raw)
	}
	return deliver, nil
}

func (s *FileStore) ReadFileRaw(rel string, params ReadFileParams) (*FileRaw, error) {
	raw, info, portable, err := s.readRaw(rel, params.MaxBytes)
	if err != nil {
		return nil, err
	}
	return &FileRaw{Path: portable, Size: info.Size(), Truncated: params.MaxBytes > 0 && int64(len(raw)) < info.Size(), Content: raw, ContentType: mimeType(portable)}, nil
}

func (s *FileStore) WriteText(rel, content string) (string, error) {
	return s.WriteTextAtomic(rel, content, true)
}

func (s *FileStore) WriteTextAtomic(rel, content string, atomic bool) (string, error) {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if atomic {
		return full, atomicWrite(full, []byte(content))
	}
	return full, os.WriteFile(full, []byte(content), 0o644)
}

func (s *FileStore) WriteJSON(rel string, value any) (string, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return s.WriteBytes(rel, append(raw, '\n'))
}

func (s *FileStore) WriteBytes(rel string, content []byte) (string, error) {
	return s.WriteBytesAtomic(rel, content, true)
}

func (s *FileStore) WriteBytesAtomic(rel string, content []byte, atomic bool) (string, error) {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if atomic {
		return full, atomicWrite(full, content)
	}
	return full, os.WriteFile(full, content, 0o644)
}

func (s *FileStore) WriteFile(rel string, content []byte) (*FileWrite, error) {
	full, err := s.WriteBytes(rel, content)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	portable, _ := s.RelativePath(full)
	return &FileWrite{Path: portable, Size: info.Size()}, nil
}

func (s *FileStore) Remove(rel string) error {
	full, err := s.ResolvePath(rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(full)
}

func (s *FileStore) DeletePath(rel string) (*PathDelete, error) {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		return nil, err
	}
	if err := s.Remove(rel); err != nil {
		return nil, err
	}
	return &PathDelete{Path: clean, Recursive: true}, nil
}

func (s *FileStore) CreateDirectory(rel string) (map[string]string, error) {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		return nil, err
	}
	_, err = s.Mkdir(clean)
	return map[string]string{"path": clean}, err
}

func (s *FileStore) Copy(fromRel, toRel string) (string, error) {
	from, err := s.ResolvePath(fromRel)
	if err != nil {
		return "", err
	}
	to, err := s.ResolvePath(toRel)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(from)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return "", err
	}
	return to, os.WriteFile(to, raw, 0o644)
}

func (s *FileStore) ReadLines(rel string, params ReadLinesParams) (*FileLines, error) {
	file, err := s.ReadFileRaw(rel, ReadFileParams{MaxBytes: params.MaxBytes})
	if err != nil {
		return nil, err
	}
	if looksBinary(file.Content) || !utf8.Valid(file.Content) {
		return nil, notTextFileError(rel)
	}
	lines := splitLines(string(file.Content))
	selected, end, err := selectLineRange(lines, params.StartLine, params.EndLine)
	if err != nil {
		return nil, err
	}
	return &FileLines{Path: file.Path, StartLine: params.StartLine, EndLine: end, TotalLines: len(lines), Lines: selected, FileTruncated: file.Truncated, Size: file.Size}, nil
}

func (s *FileStore) PatchLines(rel string, params PatchLinesParams) (*FileLinesPatch, error) {
	file, err := s.ReadFileRaw(rel, ReadFileParams{MaxBytes: params.MaxBytes})
	if err != nil {
		return nil, err
	}
	if file.Truncated {
		return nil, fileTooLargeError(rel)
	}
	if looksBinary(file.Content) || !utf8.Valid(file.Content) {
		return nil, notTextFileError(rel)
	}
	patched, end, total, err := patchLineRange(string(file.Content), params.StartLine, params.EndLine, params.Replacement)
	if err != nil {
		return nil, err
	}
	written, err := s.WriteFile(rel, []byte(patched))
	if err != nil {
		return nil, err
	}
	return &FileLinesPatch{Path: file.Path, StartLine: params.StartLine, EndLine: end, TotalLines: total, Size: written.Size}, nil
}

func (s *FileStore) Grep(params GrepParams) (*GrepResponse, error) {
	pattern := strings.TrimSpace(params.Pattern)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	path := firstNonEmpty(params.Path, ".")
	limit := firstPositiveInt(params.Limit, 200)
	maxFiles := firstPositiveInt(params.MaxFiles, 500)
	maxBytes := firstPositiveInt(params.MaxBytesPerFile, 512*1024)
	maxLine := firstPositiveInt(params.MaxLineLength, 500)
	stats, err := s.grepCandidates(path, params.Ignore)
	if err != nil {
		return nil, err
	}
	resp := &GrepResponse{Object: "list"}
	for _, item := range stats {
		if len(resp.Matches) >= limit || resp.FilesScanned >= maxFiles {
			resp.ScanTruncated = true
			break
		}
		if item.Type != FileTypeFile || item.Size > int64(maxBytes) || !isLikelyTextFile(item.Path) {
			continue
		}
		raw, err := os.ReadFile(item.FullPath)
		if err != nil || looksBinary(raw) || !utf8.Valid(raw) {
			continue
		}
		resp.FilesScanned++
		for i, line := range splitLines(string(raw)) {
			if !strings.Contains(line, pattern) {
				continue
			}
			if len(line) > maxLine {
				line = line[:maxLine]
			}
			resp.Matches = append(resp.Matches, GrepMatch{Path: item.Path, LineNumber: i + 1, Line: line})
			if len(resp.Matches) >= limit {
				resp.ScanTruncated = true
				break
			}
		}
	}
	return resp, nil
}

func (s *FileStore) grepCandidates(rel string, ignore []IgnoreRule) ([]FileStat, error) {
	full, portable, err := ResolveInside(s.Root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if portable == "" {
		portable = "."
	}
	if Ignored(portable, ignore) {
		return nil, nil
	}
	if info.IsDir() {
		return s.List(rel, ListOptions{Recursive: true, Ignore: ignore})
	}
	if info.Mode().IsRegular() {
		return []FileStat{{
			Path:       portable,
			FullPath:   full,
			Type:       FileTypeFile,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
		}}, nil
	}
	return nil, nil
}

func (s *FileStore) Summarize(params SummaryParams) (*Summary, error) {
	path := firstNonEmpty(params.Path, ".")
	maxFiles := firstPositiveInt(params.MaxFiles, 2000)
	maxPreviews := firstPositiveInt(params.MaxPreviews, 20)
	previewBytes := firstPositiveInt(params.PreviewBytes, 4096)
	topPaths := firstPositiveInt(params.TopPaths, 20)
	stats, warnings, err := s.listWithWarnings(path, ListOptions{Recursive: true, MaxDepth: params.MaxDepth, Ignore: params.Ignore})
	if err != nil {
		return nil, err
	}
	var files []FileStat
	for _, item := range stats {
		if item.Type == FileTypeFile {
			files = append(files, item)
		}
	}
	scanTruncated := len(files) > maxFiles
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	bySize := append([]FileStat(nil), files...)
	sort.Slice(bySize, func(i, j int) bool {
		if bySize[i].Size != bySize[j].Size {
			return bySize[i].Size > bySize[j].Size
		}
		return bySize[i].Path < bySize[j].Path
	})
	out := &Summary{SummaryPath: "", FileCount: len(files), GeneratedAtUnix: time.Now().Unix(), ScanTruncated: scanTruncated, ScanWarnings: warnings}
	for _, item := range files {
		out.TotalBytes += item.Size
	}
	for i, item := range bySize {
		if i >= topPaths {
			break
		}
		out.TopPathsBySize = append(out.TopPathsBySize, fmt.Sprintf("%s (%d bytes)", item.Path, item.Size))
	}
	for _, item := range bySize {
		if len(out.TextPreviews) >= maxPreviews {
			break
		}
		if !isLikelyTextFile(item.Path) || item.Size > int64(previewBytes*4) {
			continue
		}
		raw, err := os.ReadFile(item.FullPath)
		if err != nil || looksBinary(raw) || !utf8.Valid(raw) {
			continue
		}
		truncated := len(raw) > previewBytes
		if truncated {
			raw = raw[:previewBytes]
		}
		out.TextPreviews = append(out.TextPreviews, SummaryPreview{Path: item.Path, Size: item.Size, Preview: string(raw), PreviewTruncated: truncated})
	}
	return out, nil
}

func (s *FileStore) readRaw(rel string, maxBytes int) ([]byte, os.FileInfo, string, error) {
	full, portable, err := ResolveInside(s.Root, rel)
	if err != nil {
		return nil, nil, "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, "", pathError("local path is not a file", rel)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return nil, nil, "", err
	}
	if maxBytes > 0 && len(raw) > maxBytes {
		raw = raw[:maxBytes]
	}
	return raw, info, portable, nil
}

func (s *FileStore) walk(storeRoot, scanRoot, dir string, maxDepth int, opts ListOptions, out *[]FileStat, warnings *[]ScanWarning) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if dir != scanRoot && recordScanWarning(warnings, portableRel(storeRoot, dir), err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		rel, err := filepath.Rel(storeRoot, full)
		if err != nil {
			return err
		}
		portable := filepath.ToSlash(rel)
		if Ignored(portable, opts.Ignore) {
			continue
		}
		info, err := os.Lstat(full)
		if err != nil {
			if recordScanWarning(warnings, portable, err) {
				continue
			}
			return err
		}
		stat := FileStat{Path: portable, FullPath: full, Type: fileType(info), Size: info.Size(), ModifiedAt: info.ModTime()}
		if info.IsDir() {
			if opts.IncludeDirectories {
				*out = append(*out, stat)
			}
			depthRel, _ := filepath.Rel(scanRoot, full)
			depth := 1
			if depthRel != "." {
				depth = len(strings.Split(filepath.ToSlash(depthRel), "/"))
			}
			if maxDepth <= 0 || depth < maxDepth {
				if err := s.walk(storeRoot, scanRoot, full, maxDepth, opts, out, warnings); err != nil {
					return err
				}
			}
			continue
		}
		*out = append(*out, stat)
	}
	return nil
}

func fileType(info os.FileInfo) FileType {
	if info.Mode()&os.ModeSymlink != 0 {
		return FileTypeSymlink
	}
	if info.IsDir() {
		return FileTypeDirectory
	}
	if info.Mode().IsRegular() {
		return FileTypeFile
	}
	return FileTypeOther
}

func entryFromStat(item FileStat) Entry {
	size := item.Size
	if item.Type == FileTypeDirectory {
		size = 0
	}
	return Entry{Path: item.Path, IsDir: item.Type == FileTypeDirectory, Size: size, ModifiedAtUnix: item.ModifiedAt.Unix()}
}

func portableRel(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func recordScanWarning(warnings *[]ScanWarning, portable string, err error) bool {
	if !isIgnorableScanError(err) {
		return false
	}
	if len(*warnings) < maxScanWarnings {
		*warnings = append(*warnings, ScanWarning{
			Path:    firstNonEmpty(portable, "."),
			Code:    scanErrorCode(err),
			Message: err.Error(),
		})
	}
	return true
}

func isIgnorableScanError(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.ENOTDIR) ||
		errors.Is(err, syscall.ENOTCONN) ||
		errors.Is(err, syscall.ELOOP) ||
		errors.Is(err, syscall.EINVAL)
}

func scanErrorCode(err error) string {
	for _, candidate := range []struct {
		err  error
		code string
	}{
		{os.ErrNotExist, "ENOENT"},
		{os.ErrPermission, "EACCES"},
		{syscall.ENOTDIR, "ENOTDIR"},
		{syscall.ENOTCONN, "ENOTCONN"},
		{syscall.ELOOP, "ELOOP"},
		{syscall.EINVAL, "EINVAL"},
	} {
		if errors.Is(err, candidate.err) {
			return candidate.code
		}
	}
	return ""
}
