package agentapi

import (
	"strings"
	"testing"
)

type fakeKnowledgeService struct {
	calls []LocalKnowledgeSearchParams
}

func (s *fakeKnowledgeService) SearchLocalKnowledge(params LocalKnowledgeSearchParams) (LocalKnowledgeSearchResult, error) {
	s.calls = append(s.calls, params)
	return LocalKnowledgeSearchResult{
		Object: "local_knowledge_search_result",
		Data: []LocalKnowledgeHit{{
			ID:         "hit_1",
			SourceType: LocalKnowledgeSourceTranscript,
			SourceURI:  "transcript:conv:msg",
			Title:      "user message",
			Text:       "Local knowledge lives in SDKs.",
		}},
	}, nil
}

func (s *fakeKnowledgeService) CreateLocalKnowledgeContext(params LocalKnowledgeContextParams) (*LocalKnowledgeContext, error) {
	return nil, nil
}

func TestLocalKnowledgeRegistryExposesHostBackedSearchPrimitive(t *testing.T) {
	service := &fakeKnowledgeService{}
	registry := CreateLocalKnowledgeToolRegistry(service, LocalKnowledgeToolRegistryOptions{})
	definitions := registry.Definitions()
	if len(definitions) != 1 || definitions[0].Name != "local_knowledge" {
		t.Fatalf("definitions = %#v", definitions)
	}
	enum := definitions[0].Parameters["properties"].(map[string]any)["action"].(map[string]any)["enum"].([]string)
	if len(enum) != 1 || enum[0] != "search" {
		t.Fatalf("enum = %#v", enum)
	}
	if _, ok := registry.Handlers()["local_knowledge"]; !ok {
		t.Fatal("handler missing")
	}

	result, err := registry.Execute("local_knowledge", map[string]any{
		"action": "search",
		"query":  "sdk knowledge",
		"limit":  float64(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["object"] != "local_knowledge_result" || result["action"] != "search" {
		t.Fatalf("result = %#v", result)
	}
	if len(service.calls) != 1 || service.calls[0].Query != "sdk knowledge" || service.calls[0].Limit != 3 {
		t.Fatalf("calls = %#v", service.calls)
	}

	missing, err := registry.Execute("local_knowledge", map[string]any{"action": "search", "query": ""})
	if err != nil {
		t.Fatal(err)
	}
	errObj := missing["error"].(map[string]any)
	if errObj["message"] != "query is required" {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestLocalKnowledgeContextFormattingAndToolRename(t *testing.T) {
	text := FormatLocalKnowledgeContext(LocalKnowledgeContext{
		Hits: []LocalKnowledgeHit{{
			ID:         "hit_1",
			SourceType: LocalKnowledgeSourceWorkdirFile,
			SourceURI:  "file:///repo/AGENTS.md",
			Text:       "Use SDK local.",
		}},
		Text: "- AGENTS.md\n  Use SDK local.",
	})
	if !strings.Contains(text, "Local knowledge follows") || !strings.Contains(text, "AGENTS.md") {
		t.Fatalf("text = %q", text)
	}
	if LocalKnowledgeToolDefinition("project_memory").Name != "project_memory" {
		t.Fatal("renamed tool did not keep custom name")
	}
}
