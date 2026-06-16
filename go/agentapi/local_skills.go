package agentapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	DefaultFocusManifestChars = 16000
	DefaultFocusFileChars     = 12000
)

type LocalSkillDirectoryOptions struct {
	ID               string
	Name             string
	Description      string
	MaxManifestChars int
	Metadata         map[string]any
}

type LocalSkillFocusArgs struct {
	Skills           []SkillFocusItem `json:"skills"`
	MaxManifestChars int              `json:"max_manifest_chars,omitempty"`
	MaxFileChars     int              `json:"max_file_chars,omitempty"`
}

func LocalSkillFromDirectory(rootDir string, opts LocalSkillDirectoryOptions) (*LocalSkillDescriptor, error) {
	root, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	files, err := walkLocalSkillFiles(root)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	manifest, truncated, err := readLocalManifest(root, firstPositive(opts.MaxManifestChars, DefaultFocusManifestChars))
	if err != nil {
		return nil, err
	}
	base := filepath.Base(root)
	id := firstNonEmpty(opts.ID, base)
	desc := &LocalSkillDescriptor{
		LocalSkillID:      id,
		SkillRef:          skillRefForLocal(id, digest),
		Name:              firstNonEmpty(opts.Name, base),
		Description:       opts.Description,
		RootHint:          root,
		Digest:            digest,
		Manifest:          manifest,
		ManifestTruncated: truncated,
		Metadata:          opts.Metadata,
	}
	return desc, nil
}

func PendingLocalSkillCalls(response *AgentResponse) []OutputItem {
	var calls []OutputItem
	for _, call := range PendingFunctionCalls(response) {
		if call.Name == "skill_focus" {
			calls = append(calls, call)
		}
	}
	return calls
}

func RunLocalSkillHandlers(response *AgentResponse, localSkills []LocalSkillDescriptor) ([]FunctionCallOutputInput, error) {
	byRef := map[string]LocalSkillDescriptor{}
	for _, skill := range localSkills {
		ref := descriptorSkillRef(skill)
		if ref != "" {
			byRef[ref] = skill
		}
	}
	var outputs []FunctionCallOutputInput
	for _, call := range PendingLocalSkillCalls(response) {
		var args LocalSkillFocusArgs
		if call.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				return nil, err
			}
		}
		payload := focusLocalSkills(args, byRef)
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, FunctionCallOutput(call.CallID, string(raw)))
	}
	return outputs, nil
}

func focusLocalSkills(args LocalSkillFocusArgs, byRef map[string]LocalSkillDescriptor) SkillFocusResponse {
	resp := SkillFocusResponse{Object: "skill_focus_result"}
	maxManifest := firstPositive(args.MaxManifestChars, DefaultFocusManifestChars)
	maxFile := firstPositive(args.MaxFileChars, DefaultFocusFileChars)
	for _, item := range args.Skills {
		ref := ""
		if item.LocalSkill != nil {
			ref = descriptorSkillRef(*item.LocalSkill)
		}
		if ref == "" {
			ref = item.SkillID
		}
		result := SkillFocusResultItem{OK: false, SkillRef: ref, Branch: "main"}
		desc, ok := byRef[ref]
		if !ok {
			result.Error = &SkillOperationError{Code: "skill_ref_not_found", Message: "skill_ref was not registered with the SDK"}
			resp.Data = append(resp.Data, result)
			continue
		}
		skill, err := focusedLocalSkill(desc, item, maxManifest, maxFile)
		if err != nil {
			result.Error = &SkillOperationError{Code: "local_skill_focus_failed", Message: err.Error()}
			resp.Data = append(resp.Data, result)
			continue
		}
		result.OK = true
		result.LocalSkillID = desc.LocalSkillID
		result.Skill = skill
		resp.Data = append(resp.Data, result)
	}
	return resp
}

func focusedLocalSkill(desc LocalSkillDescriptor, item SkillFocusItem, maxManifest, maxFile int) (*FocusedSkill, error) {
	root, err := filepath.Abs(firstNonEmpty(desc.RootHint, "."))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local skill root is not a directory: %s", root)
	}
	includeManifest := true
	if item.IncludeManifest != nil {
		includeManifest = *item.IncludeManifest
	}
	var manifest string
	var manifestTruncated bool
	if includeManifest {
		manifest, manifestTruncated, err = readLocalManifest(root, maxManifest)
		if err != nil {
			return nil, err
		}
	}
	entries, err := localSkillEntries(root)
	if err != nil {
		return nil, err
	}
	files := make([]SkillFocusedFile, 0, len(item.Paths))
	for _, rel := range item.Paths {
		files = append(files, focusedLocalFile(root, rel, maxFile))
	}
	return &FocusedSkill{
		SkillSummary: SkillSummary{
			Object:      "focused_skill",
			SkillRef:    descriptorSkillRef(desc),
			Name:        desc.Name,
			Description: desc.Description,
			SourceType:  "SKILL_SOURCE_TYPE_LOCAL",
			Branch:      "SKILL_BRANCH_MAIN",
			Digest:      desc.Digest,
			Metadata:    desc.Metadata,
		},
		Manifest:          manifest,
		ManifestTruncated: manifestTruncated,
		Entries:           entries,
		Files:             files,
	}, nil
}

func PendingFunctionCalls(response *AgentResponse) []OutputItem {
	if response == nil {
		return nil
	}
	var calls []OutputItem
	for _, item := range response.Output {
		if item.Type == "function_call" {
			calls = append(calls, item)
		}
	}
	return calls
}

func FunctionCallOutput(callID, output string) FunctionCallOutputInput {
	return FunctionCallOutputInput{Type: "function_call_output", CallID: callID, Output: output}
}

func readLocalManifest(root string, maxChars int) (string, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if !utf8.Valid(raw) {
		return "", false, fmt.Errorf("SKILL.md must be valid UTF-8")
	}
	content, truncated := truncateRunes(string(raw), maxChars)
	return content, truncated, nil
}

func focusedLocalFile(root, relPath string, maxChars int) SkillFocusedFile {
	base := SkillFocusedFile{Path: relPath, Branch: "SKILL_BRANCH_MAIN"}
	full, ok := safeJoin(root, relPath)
	if !ok {
		base.Error = &SkillOperationError{Code: "invalid_skill_file_path", Message: "path must stay inside the local skill root"}
		return base
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		base.Error = &SkillOperationError{Code: "skill_file_not_found", Message: "skill file was not found"}
		return base
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		base.Error = &SkillOperationError{Code: "skill_file_read_failed", Message: err.Error()}
		return base
	}
	base.Size = info.Size()
	if !utf8.Valid(raw) {
		base.Error = &SkillOperationError{Code: "invalid_skill_file_utf8", Message: "skill file must be valid UTF-8"}
		return base
	}
	content, truncated := truncateRunes(string(raw), maxChars)
	base.Content = content
	base.Truncated = truncated
	return base
}

func localSkillEntries(root string) ([]SkillFileEntry, error) {
	dirents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(dirents, func(i, j int) bool { return dirents[i].Name() < dirents[j].Name() })
	var entries []SkillFileEntry
	for _, entry := range dirents {
		if ignoredArchiveDir(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		size := info.Size()
		if entry.IsDir() {
			size = 0
		}
		entries = append(entries, SkillFileEntry{
			Path:       entry.Name(),
			IsDir:      entry.IsDir(),
			Size:       size,
			ModifiedAt: info.ModTime().Unix(),
		})
	}
	return entries, nil
}

func walkLocalSkillFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && ignoredArchiveDir(d.Name()) && path != root {
			return filepath.SkipDir
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func descriptorSkillRef(desc LocalSkillDescriptor) string {
	if strings.TrimSpace(desc.SkillRef) != "" {
		return strings.TrimSpace(desc.SkillRef)
	}
	return skillRefForLocal(desc.LocalSkillID, desc.Digest)
}

func skillRefForLocal(localSkillID, digest string) string {
	slug := skillRefSlug(localSkillID)
	if slug == "" {
		slug = "local-skill"
	}
	digestPart := skillRefDigestPart(digest)
	if digestPart == "" {
		digestPart = "unknown"
	}
	return "local::" + slug + "@" + digestPart + "::main"
}

func skillRefSlug(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "::", "-")
	s = strings.ReplaceAll(s, "@", "-")
	s = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.Trim(s, "-_")
}

func skillRefDigestPart(raw string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(raw)), ":")
	s := parts[len(parts)-1]
	s = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(s, "")
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func truncateRunes(s string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s, false
	}
	return string(runes[:maxChars]), true
}

func safeJoin(root, rel string) (string, bool) {
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	relative, err := filepath.Rel(root, full)
	if err != nil {
		return "", false
	}
	return full, !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative)
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
