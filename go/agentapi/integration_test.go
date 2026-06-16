package agentapi

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func integrationClient(t *testing.T) (*Client, context.Context, context.CancelFunc) {
	t.Helper()
	if os.Getenv("AGENT_API_INTEGRATION") != "1" {
		t.Skip("set AGENT_API_INTEGRATION=1 to run live API tests")
	}
	if os.Getenv("AGENT_API_KEY") == "" {
		t.Skip("AGENT_API_KEY is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	client := NewClient(&ClientOptions{
		BaseURL:       strings.TrimRight(os.Getenv("AGENT_API_BASE_URL"), "/"),
		Timeout:       2 * time.Minute,
		StreamTimeout: 2 * time.Minute,
	})
	return client, ctx, cancel
}

func TestIntegrationDiscoveryEndpoints(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	models, err := client.Models.List(ctx)
	if err != nil {
		t.Fatalf("models.list: %v", err)
	}
	if models.Object != "list" {
		t.Fatalf("models.object = %q", models.Object)
	}

	presets, err := client.Presets.List(ctx)
	if err != nil {
		t.Fatalf("presets.list: %v", err)
	}
	if presets.Object != "list" || len(presets.Data) == 0 {
		t.Fatalf("presets response = %#v", presets)
	}

	tools, err := client.Tools.List(ctx)
	if err != nil {
		t.Fatalf("tools.list: %v", err)
	}
	if tools.Object != "list" || len(tools.Data) == 0 {
		t.Fatalf("tools response = %#v", tools)
	}
}

func TestIntegrationResponsesLifecycle(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	created, err := client.Responses.Create(ctx, ResponseCreateParams{
		Preset:          "fast-search",
		Input:           "Reply with exactly: SDK integration ok",
		MaxOutputTokens: 64,
	})
	if err != nil {
		t.Fatalf("responses.create: %v", err)
	}
	if !strings.HasPrefix(created.ID, "resp_") || created.Object != "response" {
		t.Fatalf("created response = %#v", created)
	}

	retrieved, err := client.Responses.Retrieve(ctx, created.ID)
	if err != nil {
		t.Fatalf("responses.retrieve: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Fatalf("retrieved id = %q, want %q", retrieved.ID, created.ID)
	}

	listed, err := client.Responses.List(ctx, ListResponsesParams{Limit: 5})
	if err != nil {
		t.Fatalf("responses.list: %v", err)
	}
	if listed.Object != "list" {
		t.Fatalf("listed.object = %q", listed.Object)
	}

	children, err := client.Responses.ListChildren(ctx, created.ID)
	if err != nil {
		t.Fatalf("responses.children: %v", err)
	}
	if children.Object != "list" {
		t.Fatalf("children.object = %q", children.Object)
	}

	events, err := client.Responses.ListEvents(ctx, created.ID, ListEventsParams{View: "timeline"})
	if err != nil {
		t.Fatalf("responses.events: %v", err)
	}
	if len(events.Data) == 0 {
		t.Fatalf("events were empty")
	}
}

func TestIntegrationStreamingLifecycle(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	stream, err := client.Responses.CreateStream(ctx, ResponseCreateParams{
		Preset:          "fast-search",
		Input:           "Say hi in one short sentence.",
		MaxOutputTokens: 64,
	})
	if err != nil {
		t.Fatalf("responses.create_stream: %v", err)
	}
	defer stream.Close()

	types := map[string]bool{}
	var text strings.Builder
	for stream.Next() {
		ev := stream.Event()
		types[ev.Type] = true
		text.WriteString(ev.Delta)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !(types["response.created"] || types["response.in_progress"]) {
		t.Fatalf("missing stream start event, types=%v text=%q", types, text.String())
	}
	if !(types["response.completed"] || types["response.failed"]) {
		t.Fatalf("missing stream terminal event, types=%v text=%q", types, text.String())
	}
}

func TestIntegrationAgentEndpoint(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	created, err := client.Agent.Create(ctx, ResponseCreateParams{
		Preset:          "fast-search",
		Input:           "Reply with exactly: agent endpoint ok",
		MaxOutputTokens: 64,
	})
	if err != nil {
		t.Fatalf("agent.create: %v", err)
	}
	if !strings.HasPrefix(created.ID, "resp_") {
		t.Fatalf("created id = %q", created.ID)
	}
}

func TestIntegrationSkillsReadEndpoints(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	skills, err := client.Skills.List(ctx, ListSkillsParams{Limit: 5})
	if err != nil {
		t.Fatalf("skills.list: %v", err)
	}
	if skills.Object != "list" {
		t.Fatalf("skills.object = %q", skills.Object)
	}

	discovered, err := client.Skills.Discover(ctx, DiscoverSkillsParams{Query: "test", Limit: 3})
	if err != nil {
		t.Fatalf("skills.discover: %v", err)
	}
	if discovered.Object != "list" {
		t.Fatalf("discover.object = %q", discovered.Object)
	}
}

func TestIntegrationVolumeCRUDAndFiles(t *testing.T) {
	client, ctx, cancel := integrationClient(t)
	defer cancel()

	name := "go-sdk-integration-" + time.Now().UTC().Format("20060102150405")
	volume, err := client.Volumes.Create(ctx, name)
	if err != nil {
		t.Fatalf("volumes.create: %v", err)
	}
	if volume.VolumeID == "" {
		t.Fatalf("created volume missing id: %#v", volume)
	}
	defer func() {
		if err := client.Volumes.Delete(context.Background(), volume.VolumeID); err != nil {
			t.Logf("cleanup volume %s: %v", volume.VolumeID, err)
		}
	}()

	renamed := name + "-renamed"
	updated, err := client.Volumes.Update(ctx, volume.VolumeID, renamed)
	if err != nil {
		t.Fatalf("volumes.update: %v", err)
	}
	if updated.Name != renamed {
		t.Fatalf("updated name = %q", updated.Name)
	}

	if _, err := client.Volumes.CreateDirectory(ctx, volume.VolumeID, "notes"); err != nil {
		t.Fatalf("volumes.create_directory: %v", err)
	}
	content := []byte("alpha\nneedle\nomega\n")
	if _, err := client.Volumes.WriteFile(ctx, volume.VolumeID, "notes/hello.txt", content); err != nil {
		t.Fatalf("volumes.write_file: %v", err)
	}

	read, err := client.Volumes.ReadFile(ctx, volume.VolumeID, "notes/hello.txt", ReadFileParams{MaxBytes: 1024})
	if err != nil {
		t.Fatalf("volumes.read_file: %v", err)
	}
	if !strings.Contains(read.Content, "needle") {
		t.Fatalf("read content = %#v", read)
	}

	raw, err := client.Volumes.ReadFileRaw(ctx, volume.VolumeID, "notes/hello.txt", ReadFileParams{MaxBytes: 1024})
	if err != nil {
		t.Fatalf("volumes.read_file_raw: %v", err)
	}
	if string(raw.Content) != string(content) {
		t.Fatalf("raw content = %q", string(raw.Content))
	}

	entries, err := client.Volumes.ListEntries(ctx, volume.VolumeID, VolumeEntriesParams{Path: "notes", Limit: 10})
	if err != nil {
		t.Fatalf("volumes.entries: %v", err)
	}
	if len(entries.Entries) == 0 {
		t.Fatalf("entries were empty")
	}

	grep, err := client.Volumes.Grep(ctx, volume.VolumeID, VolumeEntriesParams{Path: "notes", Query: "needle", Limit: 10})
	if err != nil {
		t.Fatalf("volumes.grep: %v", err)
	}
	if len(grep.Matches) == 0 {
		t.Fatalf("grep returned no matches")
	}

	lines, err := client.Volumes.ReadLines(ctx, volume.VolumeID, "notes/hello.txt", ReadLinesParams{StartLine: 1, EndLine: 2, MaxBytes: 1024})
	if err != nil {
		t.Fatalf("volumes.read_lines: %v", err)
	}
	if len(lines.Lines) == 0 {
		t.Fatalf("read_lines returned no lines")
	}

	if _, err := client.Volumes.PatchLines(ctx, volume.VolumeID, "notes/hello.txt", PatchLinesParams{StartLine: 2, EndLine: 2, Replacement: "patched\n"}); err != nil {
		t.Fatalf("volumes.patch_lines: %v", err)
	}

	archive, err := client.Volumes.DownloadArchive(ctx, volume.VolumeID, "notes")
	if err != nil {
		t.Fatalf("volumes.archive: %v", err)
	}
	if len(archive.Content) == 0 {
		t.Fatalf("archive was empty")
	}

	if _, err := client.Volumes.Summarize(ctx, volume.VolumeID, "notes"); err != nil {
		t.Fatalf("volumes.summarize: %v", err)
	}

	if _, err := client.Volumes.ReconcileUsage(ctx, volume.VolumeID); err != nil {
		t.Fatalf("volumes.reconcile_usage: %v", err)
	}
}
