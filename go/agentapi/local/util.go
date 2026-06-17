package local

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

func atomicWrite(path string, content []byte) error {
	tmp := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s.%d.tmp", filepath.Base(path), os.Getpid()))
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func looksBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

func mimeType(rel string) string {
	ext := strings.ToLower(filepath.Ext(rel))
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	if isLikelyTextFile(rel) {
		return "text/plain"
	}
	return "application/octet-stream"
}

func isLikelyTextFile(rel string) bool {
	lower := strings.ToLower(rel)
	ext := filepath.Ext(lower)
	if ext == "" || strings.HasSuffix(lower, "dockerfile") {
		return true
	}
	_, ok := textExtensions[ext]
	return ok
}

var textExtensions = map[string]struct{}{
	".txt": {}, ".md": {}, ".markdown": {}, ".json": {}, ".yaml": {}, ".yml": {}, ".toml": {}, ".xml": {}, ".csv": {},
	".py": {}, ".go": {}, ".js": {}, ".mjs": {}, ".cjs": {}, ".ts": {}, ".tsx": {}, ".jsx": {}, ".html": {}, ".htm": {},
	".css": {}, ".scss": {}, ".sh": {}, ".bash": {}, ".zsh": {}, ".sql": {}, ".env": {}, ".ini": {}, ".cfg": {},
	".conf": {}, ".log": {}, ".rst": {}, ".adoc": {}, ".gradle": {}, ".properties": {}, ".mod": {}, ".sum": {}, ".dockerfile": {},
}

func splitLines(content string) []string {
	if content == "" {
		return []string{""}
	}
	if strings.HasSuffix(content, "\n") {
		content = strings.TrimSuffix(content, "\n")
		if content == "" {
			return []string{""}
		}
	}
	return strings.Split(content, "\n")
}

func selectLineRange(lines []string, startLine, endLine int) ([]string, int, error) {
	if startLine < 1 {
		return nil, 0, fmt.Errorf("startLine must be >= 1")
	}
	total := len(lines)
	if total == 0 {
		total = 1
	}
	end := endLine
	if end <= 0 || end > total {
		end = total
	}
	if startLine > total || end < startLine {
		return nil, 0, fmt.Errorf("invalid line range")
	}
	return lines[startLine-1 : end], end, nil
}

func patchLineRange(content string, startLine, endLine int, replacement string) (string, int, int, error) {
	lines := splitLines(content)
	_, end, err := selectLineRange(lines, startLine, endLine)
	if err != nil {
		return "", 0, 0, err
	}
	var repl []string
	if replacement != "" {
		repl = splitLines(replacement)
	}
	patched := append([]string{}, lines[:startLine-1]...)
	patched = append(patched, repl...)
	patched = append(patched, lines[end:]...)
	out := strings.Join(patched, "\n")
	if strings.HasSuffix(content, "\n") {
		out += "\n"
	}
	return out, end, len(patched), nil
}

func firstPositiveInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(query)))
}

func snapshotFileChanged(before, after SnapshotFile) bool {
	if before.SHA256 != "" || after.SHA256 != "" {
		return before.SHA256 != after.SHA256
	}
	return before.Size != after.Size || before.ModifiedAtUnix != after.ModifiedAtUnix
}

func matchesPathFilters(rel string, include, exclude []string) bool {
	if len(include) > 0 {
		ok := false
		for _, pattern := range include {
			if pathMatches(rel, pattern) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, pattern := range exclude {
		if pathMatches(rel, pattern) {
			return false
		}
	}
	return true
}

func pathMatches(rel, pattern string) bool {
	clean := strings.TrimLeft(strings.ReplaceAll(pattern, "\\", "/"), "/")
	if clean == "" || clean == "." {
		return true
	}
	if !strings.Contains(clean, "*") {
		return rel == clean || strings.HasPrefix(rel, clean+"/")
	}
	ok, _ := filepath.Match(clean, rel)
	return ok
}

func ignoreGlobs(patterns []string) []IgnoreRule {
	out := make([]IgnoreRule, 0, len(patterns))
	for _, pattern := range patterns {
		out = append(out, IgnoreGlob(pattern))
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
