package local

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAppDirsLinuxXDG(t *testing.T) {
	dirs, err := AppDirsFor("Agent Studio", AppDirsOptions{
		Platform: "linux",
		Env: map[string]string{
			"HOME":            "/home/dev",
			"XDG_DATA_HOME":   "/xdg/data",
			"XDG_CONFIG_HOME": "/xdg/config",
			"XDG_CACHE_HOME":  "/xdg/cache",
			"XDG_STATE_HOME":  "/xdg/state",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dirs.Data != "/xdg/data/agent-studio" || dirs.Config != "/xdg/config/agent-studio" || dirs.Cache != "/xdg/cache/agent-studio" || dirs.Logs != "/xdg/state/agent-studio/logs" {
		t.Fatalf("dirs = %#v", dirs)
	}
}

func TestFileStoreTraversalAndWorkbenchOps(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolvePath("../outside.txt"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := store.ResolvePath("/tmp/outside.txt"); err == nil {
		t.Fatal("expected absolute path rejection")
	}
	if _, err := store.WriteText("notes/hello.md", "# Hello\nneedle one\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteBytes("assets/blob.bin", []byte{0, 1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.ListEntries(".", ListOptions{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEntry(entries.Entries, "notes/hello.md") {
		t.Fatalf("entries = %#v", entries.Entries)
	}
	search, err := store.SearchEntries("hello", ".", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Entries) != 1 || search.Entries[0].Path != "notes/hello.md" {
		t.Fatalf("search = %#v", search.Entries)
	}
	text, err := store.ReadFile("notes/hello.md", ReadFileParams{})
	if err != nil {
		t.Fatal(err)
	}
	if text.Encoding != "text" || !strings.Contains(text.Content, "needle one") {
		t.Fatalf("text = %#v", text)
	}
	binary, err := store.ReadFile("assets/blob.bin", ReadFileParams{})
	if err != nil {
		t.Fatal(err)
	}
	if binary.Encoding != "base64" || binary.ContentBase64 != "AAECAw==" {
		t.Fatalf("binary = %#v", binary)
	}
}

func TestFileStoreLinesGrepSummary(t *testing.T) {
	store, _ := NewFileStore(t.TempDir(), "")
	_, _ = store.WriteText("src/a.go", "a\nneedle\nc\n")
	_, _ = store.WriteText("src/b.go", "again needle\n")
	_, _ = store.WriteBytes("src/blob.bin", []byte{0, 'n', 'e', 'e', 'd', 'l', 'e'})
	lines, err := store.ReadLines("src/a.go", ReadLinesParams{StartLine: 2, EndLine: 3})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines.Lines, ",") != "needle,c" {
		t.Fatalf("lines = %#v", lines.Lines)
	}
	patch, err := store.PatchLines("src/a.go", PatchLinesParams{StartLine: 2, EndLine: 2, Replacement: "NEEDLE\nNEEDLE2"})
	if err != nil {
		t.Fatal(err)
	}
	if patch.TotalLines != 4 {
		t.Fatalf("patch = %#v", patch)
	}
	grep, err := store.Grep(GrepParams{Pattern: "needle", Path: "src", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if grep.FilesScanned != 2 || len(grep.Matches) != 1 || grep.Matches[0].Path != "src/b.go" {
		t.Fatalf("grep = %#v", grep)
	}
	summary, err := store.Summarize(SummaryParams{MaxPreviews: 2})
	if err != nil {
		t.Fatal(err)
	}
	if summary.FileCount != 3 || len(summary.TextPreviews) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestFileStoreSkipsBrokenSymlinksDuringRecursiveScans(t *testing.T) {
	root := t.TempDir()
	store, _ := NewFileStore(root, "")
	_, _ = store.WriteText("README.md", "# Project\nneedle\n")
	if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "SingletonCookie")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	files, err := store.List(".", ListOptions{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != "README.md" || files[0].Type != FileTypeFile || files[1].Path != "SingletonCookie" || files[1].Type != FileTypeSymlink {
		t.Fatalf("files = %#v", files)
	}
	summary, err := store.Summarize(SummaryParams{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.FileCount != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	grep, err := store.Grep(GrepParams{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(grep.Matches) != 1 || grep.Matches[0].Path != "README.md" {
		t.Fatalf("grep = %#v", grep)
	}
}

func TestWorkdirIgnoresEditsSnapshotsAndContext(t *testing.T) {
	root := t.TempDir()
	workdir, err := NewWorkdir(root, WorkdirOptions{Name: "Demo", Trusted: true, Ignore: []IgnoreRule{IgnoreRegexp(regexp.MustCompile("ignored"))}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workdir.WriteText(".gitignore", "ignored-dir/\n*.tmp\n"); err != nil {
		t.Fatal(err)
	}
	_, _ = workdir.Files.WriteText("ignored-dir/a.txt", "hidden\n")
	_, _ = workdir.Files.WriteText("keep.tmp", "hidden\n")
	if _, err := workdir.WriteText("src/index.go", "a\nb\nc\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := workdir.LoadIgnoreFiles(); err != nil {
		t.Fatal(err)
	}
	entries, err := workdir.ListEntries(".", ListOptions{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if hasEntry(entries.Entries, "ignored-dir/a.txt") || hasEntry(entries.Entries, "keep.tmp") {
		t.Fatalf("ignored entries leaked: %#v", entries.Entries)
	}
	if _, err := workdir.ResolvePath("ignored-dir/a.txt"); err == nil {
		t.Fatal("expected ignored path rejection")
	}
	preview, err := workdir.PreviewPatchLines("src/index.go", PatchLinesParams{StartLine: 2, EndLine: 2, Replacement: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(preview.Before, ",") != "b" || strings.Join(preview.After, ",") != "B" {
		t.Fatalf("preview = %#v", preview)
	}
	before, err := workdir.Snapshot(SnapshotParams{})
	if err != nil {
		t.Fatal(err)
	}
	hash := ""
	for _, file := range before.Files {
		if file.Path == "src/index.go" {
			hash = file.SHA256
		}
	}
	plan, err := workdir.PreviewEdits([]LineEdit{{Path: "src/index.go", StartLine: 2, EndLine: 2, Replacement: "B", ExpectedSHA256: hash}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := workdir.ApplyEdits(plan.Edits)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "src/index.go" || result.EditCount != 1 {
		t.Fatalf("result = %#v", result)
	}
	content, _ := workdir.ReadText("src/index.go")
	if content != "a\nB\nc\n" {
		t.Fatalf("content = %q", content)
	}
	after, _ := workdir.Snapshot(SnapshotParams{})
	diff := workdir.Diff(before, after)
	if len(diff.Modified) != 1 || diff.Modified[0].After.Path != "src/index.go" {
		t.Fatalf("diff = %#v", diff)
	}
	_, _ = workdir.WriteText("README.md", "# Demo\nneedle\n")
	_, _ = workdir.WriteText(".env", "TOKEN=secret\n")
	manifest, err := CreateContextPackage(workdir, ContextPackageParams{Query: "needle", IncludeSearch: true, MaxFiles: 10, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Object != "local_context_manifest" || manifest.WorkdirName != "Demo" || manifest.Summary == nil || manifest.Search == nil {
		t.Fatalf("manifest = %#v", manifest)
	}
	var envFile *ContextFile
	for i := range manifest.Files {
		if manifest.Files[i].Path == ".env" {
			envFile = &manifest.Files[i]
		}
	}
	if envFile == nil || envFile.Sensitivity != SensitivitySecret || envFile.OmittedReason != "secret_path" || envFile.Content != "" {
		t.Fatalf("env file = %#v", envFile)
	}
}

func TestRuntimeConfigAndSkills(t *testing.T) {
	root := t.TempDir()
	runtime, err := NewRuntime(RuntimeOptions{AppName: "agent-studio", BaseDir: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Config.Set("settings.json", "baseURL", "https://agent.test"); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.Config.Get("settings.json", "baseURL", "")
	if err != nil || value != "https://agent.test" {
		t.Fatalf("config value=%v err=%v", value, err)
	}
	skillDir := filepath.Join(runtime.Data.Root, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, err := runtime.Skills.Discover(SkillDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].LocalSkillID != "demo" {
		t.Fatalf("skills = %#v", skills)
	}
}

func hasEntry(entries []Entry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}
