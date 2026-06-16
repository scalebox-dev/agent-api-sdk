package agentapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalSkillFromDirectoryAndFocus(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Demo\nUse this skill."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflow.md"), []byte("Step 1\nStep 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	desc, err := LocalSkillFromDirectory(root, LocalSkillDirectoryOptions{ID: "Demo Skill"})
	if err != nil {
		t.Fatal(err)
	}
	if desc.SkillRef == "" || !strings.HasPrefix(desc.SkillRef, "local::demo-skill@") {
		t.Fatalf("SkillRef = %q", desc.SkillRef)
	}
	resp := &AgentResponse{Output: []OutputItem{{
		Type:      "function_call",
		CallID:    "call_1",
		Name:      "skill_focus",
		Arguments: `{"skills":[{"local_skill":{"skill_ref":"` + desc.SkillRef + `"},"paths":["workflow.md"]}]}`,
	}}}
	outputs, err := RunLocalSkillHandlers(resp, []LocalSkillDescriptor{*desc})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 || outputs[0].CallID != "call_1" {
		t.Fatalf("outputs = %#v", outputs)
	}
	if !strings.Contains(outputs[0].Output, "Step 1") {
		t.Fatalf("output payload = %s", outputs[0].Output)
	}
}

func TestZipDirectoryAndExtract(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive, err := ZipDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	result, err := ExtractZipToDirectory(archive, target, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.FileCount != 1 || result.ByteCount != 7 {
		t.Fatalf("result = %#v", result)
	}
	raw, err := os.ReadFile(filepath.Join(target, "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "content" {
		t.Fatalf("extracted = %q", string(raw))
	}
}
