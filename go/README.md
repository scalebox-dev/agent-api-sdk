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

The SDK includes retries, typed API errors, timeouts, SSE streaming, durable volume APIs, and skill APIs.

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
