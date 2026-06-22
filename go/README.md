# Go SDK

Production Go SDK for the Managed Agent API. The module is pinned to Go 1.24.

## Install

```bash
go get github.com/scalebox-dev/agent-api-sdk/go@latest
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/scalebox-dev/agent-api-sdk/go/agentapi"
)

func main() {
	client := agentapi.NewClient(nil)
	resp, err := client.Responses.Create(context.Background(), agentapi.ResponseCreateParams{
		Preset: "fast-search",
		Input:  "Say hello in one short sentence.",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.OutputText)
}
```

`NewClient(nil)` reads `AGENT_API_KEY` and `AGENT_API_BASE_URL` from the environment. The default base URL is `https://api.agentsway.dev`.

## Resources

- `client.Responses` / `client.Agent`
- `client.Models`
- `client.Presets`
- `client.Tools`
- `client.Volumes`
- `client.Skills`
- `client.Auth`

The SDK includes retries, typed API errors, timeouts, SSE streaming, durable volume APIs, and skill APIs.
Use `context.Context` to cancel local SDK calls and `client.Responses.Cancel(ctx, responseID)` for backend best-effort cancellation after a response ID exists.

## Preset tools and local/client tools

`Tools` is the concrete model-visible tool list. Tool names must be unique because model tool calls select tools by name. When you send `Preset` and `Tools` together, the explicit `Tools` slice replaces the preset's default tools. Hybrid apps that add local function tools should resolve the preset defaults first, merge in their local tools, then pass the merged slice. The SDK rejects duplicate tool names before submitting requests.

```go
workdir, err := local.NewWorkdir("/path/to/project", local.WorkdirOptions{Trusted: true})
if err != nil {
	log.Fatal(err)
}
localTools := local.CreateWorkdirToolRegistry(workdir, agentapi.LocalWorkdirToolRegistryOptions{
	AccessMode: agentapi.LocalWorkdirAccessApproval,
})
shellTools, err := local.CreateShellToolRegistry(workdir, agentapi.LocalShellToolRegistryOptions{
	AccessMode: agentapi.LocalShellAccessApproval,
})
if err != nil {
	log.Fatal(err)
}

resolved, err := agentapi.ResolvePresetTools(ctx, client, agentapi.ResolvePresetToolsOptions{
	Preset: "pro-search",
	Tools:  append(localTools.Definitions(), shellTools.Definitions()...),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Agent.Create(ctx, agentapi.ResponseCreateParams{
	Preset: "pro-search",
	Input:  "Research the topic and update local notes.",
	Tools:  resolved.Tools,
})
```

For long-running apps, cache `client.Presets.List()` and `client.Tools.List()` and refresh them periodically. Use `ResolvePresetToolsFromCatalog()` with cached catalogs when you want deterministic request construction without fetching on every turn.

## Browser Device Login

CLI and desktop apps can use browser login without handling user passwords or static API keys.

```go
challenge, err := client.Auth.StartDeviceAuth(ctx, agentapi.StartDeviceAuthParams{
	ClientName: "Agent CLI",
})
if err != nil {
	log.Fatal(err)
}
fmt.Println("Open", challenge.VerificationURIComplete)

session, err := client.Auth.WaitForDeviceAuth(ctx, agentapi.PollDeviceAuthParams{
	DeviceCode: challenge.DeviceCode,
}, agentapi.WaitForDeviceAuthOptions{
	Interval: time.Duration(challenge.IntervalSeconds) * time.Second,
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(session.AccessToken)
```

The SDK returns URLs and polling helpers only. Opening the browser belongs to the CLI, Electron, Tauri, or native host app.

Long-running local apps can keep browser sessions fresh explicitly:

```go
if agentapi.BrowserAuthSessionExpiresWithin(session, 5*time.Minute, time.Now()) {
	session, err = client.Auth.RefreshBrowserSession(ctx, agentapi.RefreshBrowserSessionParams{
		RefreshToken: session.RefreshToken,
	})
	if err != nil {
		log.Fatal(err)
	}
	// Persist the refreshed session in your app's secure profile store.
}
```

## Local Skills

```go
skill, err := agentapi.LocalSkillFromDirectory("./my-skill", agentapi.LocalSkillDirectoryOptions{})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Responses.Create(ctx, agentapi.ResponseCreateParams{
	Input:       "Use the local skill.",
	LocalSkills: []agentapi.LocalSkillDescriptor{*skill},
})
```

## Local Runtime

Local app and CLI integrations can use the `agentapi/local` subpackage for framework-neutral filesystem and workdir support. It is not a desktop UI kit; Electron, Qt, Tauri, or native apps should keep UI policy in their host framework and call this layer from a trusted local process.

```go
package main

import (
	"log"

	local "github.com/scalebox-dev/agent-api-sdk/go/agentapi/local"
)

func main() {
	rt, err := local.NewRuntime(local.RuntimeOptions{AppName: "agent-studio"})
	if err != nil {
		log.Fatal(err)
	}
	if err := rt.Ensure(); err != nil {
		log.Fatal(err)
	}

	if _, err := rt.Config.Set("settings.json", "baseURL", "https://api.agentsway.dev"); err != nil {
		log.Fatal(err)
	}
	if _, err := rt.Cache.WriteJSON("models.json", []map[string]string{{"id": "openai/gpt-5.5"}}); err != nil {
		log.Fatal(err)
	}

	project, err := rt.Workdir("/path/to/project", local.WorkdirOptions{Name: "my-project", Trusted: true})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := project.LoadIgnoreFiles(); err != nil {
		log.Fatal(err)
	}

	matches, err := project.Grep(local.GrepParams{Pattern: "billing", Path: "src"})
	if err != nil {
		log.Fatal(err)
	}
	_ = matches

	before, err := project.Snapshot(local.SnapshotParams{})
	if err != nil {
		log.Fatal(err)
	}
	plan, err := project.PreviewEdits([]local.LineEdit{{
		Path:        "src/app.go",
		StartLine:   1,
		EndLine:     1,
		Replacement: "fmt.Println(\"patched\")",
	}})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := project.ApplyEdits(plan.Edits); err != nil {
		log.Fatal(err)
	}
	after, err := project.Snapshot(local.SnapshotParams{})
	if err != nil {
		log.Fatal(err)
	}
	diff := project.Diff(before, after)
	_ = diff

	context, err := local.CreateContextPackage(project, local.ContextPackageParams{
		Query:         "billing",
		IncludeSearch: true,
		MaxFiles:      80,
		MaxBytes:      256 * 1024,
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = context
}
```

The local runtime provides cross-platform app directories, root-scoped file stores, atomic text/JSON/byte writes, workbench-style entry search and file delivery, line edits, grep, summaries, default workdir ignore rules, `.gitignore` loading, snapshots, diffs, conflict-aware multi-file edits with rollback, local skill discovery, sensitivity classification, and bounded context packages for agent handoff.

## Streaming

```go
stream, err := client.Responses.CreateStream(ctx, agentapi.ResponseCreateParams{
	Input:  "Tell me a story.",
	Stream: true,
})
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for stream.Next() {
	ev := stream.Event()
	if ev.Type == "response.output_text.delta" {
		fmt.Print(ev.Delta)
	}
}
if err := stream.Err(); err != nil {
	log.Fatal(err)
}
```

## Local Checks

```bash
GOWORK=off go test ./...
GOWORK=off go run ./scripts/check_routes.go
```

Live integration tests are opt-in:

```bash
AGENT_API_INTEGRATION=1 \
AGENT_API_KEY=sk-... \
AGENT_API_BASE_URL=https://api.agentsway.dev \
GOWORK=off go test ./agentapi -run Integration -count=1 -v
```
