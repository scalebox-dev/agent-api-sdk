# JavaScript SDK

Production JavaScript/TypeScript SDK for the Managed Agent API.

**Published on npm:** [`@agent-api/sdk`](https://www.npmjs.com/package/@agent-api/sdk) (v1.0.8)

## Install

```bash
npm install @agent-api/sdk
```

For local development from this repository:

```bash
cd sdk/javascript
npm install
npm run build
```

## Quick start

```javascript
import { AgentAPI } from "@agent-api/sdk";

const client = new AgentAPI({
  apiKey: process.env.AGENT_API_KEY,
  baseURL: "https://api.agentsway.dev",
});

const response = await client.responses.create({
  preset: "pro-search",
  input: "What changed in AI this week?",
});
console.log(response.output_text);
```

Environment variables `AGENT_API_KEY` and `AGENT_API_BASE_URL` are used by default.
The default base URL is `https://api.agentsway.dev` when neither option nor env is set.

## Package layout

```
src/
  client.ts           # AgentAPI entrypoint
  errors.ts           # Typed error hierarchy + retry helpers
  pagination.ts       # Cursor pagination utilities
  streaming.ts        # SSE parser
  resources/          # responses, models, presets, tools, volumes, skills
  types/              # Request/response TypeScript types
  internal/http.ts    # Retries, timeouts, User-Agent
```

## Resources

| Resource | Methods |
|----------|---------|
| `client.responses` / `client.agent` | `create`, `list`, `listPage`, `listIterator`, `retrieve`, `cancel`, `listChildren`, `listEvents` |
| `client.models` | `list` |
| `client.presets` | `list` |
| `client.tools` | `list` |
| `client.volumes` | `list`, `create`, `retrieve`, `update`, `delete`, `listEntries`, `searchEntries`, `readFile`, `writeFile`, `deletePath`, `reconcileUsage`, `createDirectory`, `downloadArchive`, `summarize`, `readLines`, `patchLines`, `grep` |
| `client.skills` | `list`, `create`, `discover`, `focus`, `createDev`, `updateFile`, `retrieve`, `update`, `archive`, `delete`, `diff`, `acceptDev`, `discardDev`, `exportArchive`, `importArchive`, `pushDirectory`, `pullDirectory`, `listFiles`, `readFile`, `writeFile`, `deleteFile` |

## Durable Volumes

```typescript
const volume = await client.volumes.create({ name: "research-notes" });

await client.volumes.writeFile(volume.volume_id, "notes/summary.md", "# Summary\n");
const file = await client.volumes.readFile(volume.volume_id, "notes/summary.md");
const binary = await client.volumes.readFile(volume.volume_id, "assets/logo.png", { format: "raw" });

const response = await client.agent.create({
  preset: "pro-search",
  input: "Use the attached workspace volume.",
  volume_id: volume.volume_id,
});
```

## Skills

`localSkillFromDirectory()` reads `SKILL.md` into the descriptor for initial local-skill auto-focus; later focused reads still use the local skill tool bridge.

```typescript
import { localSkillFromDirectory } from "@agent-api/sdk";

const skill = await client.skills.create({ name: "research-helper" });
await client.skills.writeFile(skill.skill_id, "SKILL.md", "# Research helper\n");

const localSkill = await localSkillFromDirectory("./skills/research-helper");
const response = await client.responses.create({
  input: "Use the research helper.",
  skills: [{ skill_id: skill.skill_id, branch: "main" }],
  local_skills: [localSkill],
});
```

## Production features

- **Retries:** automatic exponential backoff for network failures, 429, and 5xx (default 2 retries).
- **Timeouts:** 10 minute default for requests; 1 hour for streaming agent runs (configurable via `timeout` / `streamTimeout`).
- **Typed errors:** `AuthenticationError`, `RateLimitError`, `NotFoundError`, `BadRequestError`, etc.
- **Pagination:** `listPage` returns a cursor page; `listIterator` auto-fetches all pages.

```typescript
for await (const item of client.responses.listIterator({ limit: 20 })) {
  console.log(item.id, item.status);
}
```

## Streaming

```typescript
const stream = await client.responses.create({
  preset: "fast-search",
  input: "Summarize today's AI news.",
  stream: true,
});

for await (const event of stream) {
  if (event.type === "response.output_text.delta") {
    process.stdout.write(event.delta ?? "");
  }
}
```

## Model routing

Use `model_routing: "auto"` to let the platform pick a model (openmark). Optionally constrain the pool with `models`, and tune selection with `routing_strategy`:

```typescript
const response = await client.responses.create({
  input: "Compare two cloud providers for ML workloads.",
  model_routing: "auto",
  routing_strategy: "cost-effective",
  models: ["openai/gpt-5.4", "google/gemini-3-flash-preview"],
});
```

Use model ids in vendor/model form (values from `client.models.list()`). Omit `model_routing` (or set `"chain"`) for strict fallback order via `model` / `models`. `routing_strategy` is only valid when `model_routing` is `"auto"`.

## Client options

```typescript
const client = new AgentAPI({
  apiKey: "sk-...",
  baseURL: "https://api.agentsway.dev",
  timeout: 600_000,
  streamTimeout: 3_600_000,
  maxRetries: 2,
  defaultHeaders: { "X-Custom": "value" },
});
```

## Tests

```bash
npm run check:routes   # optional route manifest check
npm test
AGENT_API_INTEGRATION=1 AGENT_API_KEY=sk-... AGENT_API_BASE_URL=https://api.agentsway.dev npm run test:integration
```

## Scope

The SDK covers the public agent/Responses API, durable volume APIs, skill APIs, and discovery
endpoints. Console auth, workspace administration, and internal audit records
are intentionally out of scope.
