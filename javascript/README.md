# JavaScript SDK

Production JavaScript/TypeScript SDK for the Managed Agent API.

**Published on npm:** [`@agent-api/sdk`](https://www.npmjs.com/package/@agent-api/sdk) (v1.2.2)

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

Public package entrypoints:

- `@agent-api/sdk`: browser-safe REST client, public types, auth, responses, models, presets, tools, volumes, and skills HTTP APIs.
- `@agent-api/sdk/browser`: explicit alias of the browser-safe REST client entry.
- `@agent-api/sdk/local`: Node-only local runtime, workdir, context, local workdir tools, and local shell tools.
- `@agent-api/sdk/node`: Node aggregate entry for local helpers such as `localSkillFromDirectory()` plus `NodeAgentAPI`.

```
src/
  client.ts           # AgentAPI entrypoint
  errors.ts           # Typed error hierarchy + retry helpers
  pagination.ts       # Cursor pagination utilities
  streaming.ts        # SSE parser
  resources/          # auth, responses, models, presets, tools, volumes, skills
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
| `client.skills` | `list`, `create`, `discover`, `focus`, `createDev`, `updateFile`, `retrieve`, `update`, `archive`, `delete`, `diff`, `acceptDev`, `discardDev`, `exportArchive`, `importArchive`, `listFiles`, `readFile`, `writeFile`, `deleteFile` |
| `client.auth` | `startDeviceAuth`, `pollDeviceAuth`, `waitForDeviceAuth`, `refreshBrowserSession` |

`NodeAgentAPI` from `@agent-api/sdk/node` extends `client.skills` with Node-only `pushDirectory` and `pullDirectory`.

## Browser Device Login

CLI and desktop apps can use browser login without handling user passwords or static API keys.

```typescript
const challenge = await client.auth.startDeviceAuth({ client_name: "Agent CLI" });
console.log(`Open ${challenge.verification_uri_complete}`);

const session = await client.auth.waitForDeviceAuth({
  device_code: challenge.device_code,
  interval_seconds: challenge.interval_seconds,
});

console.log(session.access_token);
```

The SDK returns URLs and polling helpers only. Opening the browser belongs to the CLI, Electron, Tauri, or native host app.

Long-running local apps can keep browser sessions fresh explicitly:

```typescript
import { browserAuthSessionExpiresWithin } from "@agent-api/sdk";

if (browserAuthSessionExpiresWithin(session, 5 * 60_000)) {
  session = await client.auth.refreshBrowserSession({
    refresh_token: session.refresh_token,
  });
  // Persist the refreshed session in your app's secure profile store.
}
```

## Durable Volumes

```typescript
const volume = await client.volumes.create({ name: "research-notes" });

await client.volumes.writeFile(volume.volume_id, "notes/summary.md", "# Summary\n");
const file = await client.volumes.readFile(volume.volume_id, "notes/summary.md");
const binary = await client.volumes.readFile(volume.volume_id, "assets/logo.png", { format: "raw" });

const response = await client.agent.create({
  preset: "pro-search",
  input: "Use the attached agent volume.",
  volume_id: volume.volume_id,
});
```

## Preset tools and local/client tools

`tools` is the concrete model-visible tool list. Tool names must be unique because model tool calls select tools by name. When you send `preset` and `tools` together, the explicit `tools` array replaces the preset's default tools. Hybrid apps that add local function tools should resolve the preset defaults first, merge in their local tools, then pass the merged array. The SDK rejects duplicate tool names before submitting requests.

```typescript
import { resolvePresetTools } from "@agent-api/sdk";
import { createLocalShellToolRegistry, createLocalWorkdirToolRegistry, LocalWorkdir } from "@agent-api/sdk/local";

const workdir = new LocalWorkdir("/path/to/project", { trusted: true });
const workdirRegistry = createLocalWorkdirToolRegistry(workdir);
const shellRegistry = createLocalShellToolRegistry({ workdir });

const { tools } = await resolvePresetTools(client, {
  preset: "pro-search",
  tools: [...workdirRegistry.definitions(), ...shellRegistry.definitions()],
});

const response = await client.agent.create({
  preset: "pro-search",
  input: "Research the topic and update local notes.",
  tools,
});
```

For long-running apps, cache `client.presets.list()` and `client.tools.list()` and refresh them periodically. Use `resolvePresetToolsFromCatalog()` with cached catalogs when you want deterministic request construction without fetching on every turn.

## Skills

`localSkillFromDirectory()` reads `SKILL.md` into the descriptor for initial local-skill auto-focus; later focused reads still use the local skill tool bridge.

```typescript
import { localSkillFromDirectory } from "@agent-api/sdk/node";

const skill = await client.skills.create({ name: "research-helper" });
await client.skills.writeFile(skill.skill_id, "SKILL.md", "# Research helper\n");

const localSkill = await localSkillFromDirectory("./skills/research-helper");
const response = await client.responses.create({
  input: "Use the research helper.",
  skills: [{ skill_id: skill.skill_id, branch: "main" }],
  local_skills: [localSkill],
});
```

## Local Runtime

Local app integrations can use the Node-only `@agent-api/sdk/local` entrypoint for core filesystem/runtime support. This layer is framework-neutral: it does not depend on Electron, Tauri, native UI kits, or browser APIs.

```typescript
import { createLocalRuntime } from "@agent-api/sdk/local";

const local = createLocalRuntime({
  appName: "agent-studio",
});

await local.ensure();

await local.config.set("settings.json", "baseURL", "https://api.agentsway.dev");
const baseURL = await local.config.get<string>("settings.json", "baseURL");

await local.cache.writeJSON("models.json", [{ id: "openai/gpt-5.5" }]);
const skills = await local.skills.discover();
```

The runtime provides:

- Cross-platform app directories for data, config, cache, logs, and temp files.
- Root-scoped file stores with path traversal protection.
- Atomic text/JSON writes, byte reads/writes, recursive listing, copy, remove, and stat helpers.
- Local workdir operations inspired by platform volumes: `listEntries`, `searchEntries`, `readFile`, `writeFile`, `deletePath`, `createDirectory`, `readLines`, `patchLines`, `grep`, and `summarize`.
- First-class local workdirs with default ignore rules, scoped workbench operations, patch previews, snapshots, diffs, file-watch handles, and budgeted context packaging.
- Model-facing local tools for workdir operations (`local_workdir`) and command execution (`local_shell`) with approval/full-access dispatch conventions.
- Typed local errors, `.gitignore` loading, sensitivity classification, and multi-file edit plans with rollback on failure.
- JSON config helpers for typed app settings.
- Local skill discovery built on `localSkillFromDirectory()`.

Keep UI and OS interaction policy in your host framework. Electron, Tauri, Qt, or native apps should call this layer from their trusted local process and expose only the capabilities their UI needs.

`local_shell` executes commands through a pluggable command runner. The default `HostLocalShellRunner` runs on the user's local machine, bounds captured output, enforces a timeout, and confines relative `workdir` overrides under the configured cwd. It is not a security sandbox; use approval mode unless your application intentionally grants full local command access.

For OS-level isolation, install the standalone `agent-isolator` binary from the
GitHub Release artifacts and make it available on `PATH`. `isolation: "auto"`
tries `agent-isolator` first and falls back to direct execution if it is missing
or unavailable. `isolation: "required"` fails closed when an isolating runner
cannot be selected. The SDK does not run postinstall scripts or build native
binaries during package installation.

```typescript
const workdir = local.data.child("workdirs/demo");

await workdir.writeText("src/index.ts", "console.log('hello');\n");

const matches = await workdir.grep({ pattern: "hello", path: "src" });
const lines = await workdir.readLines("src/index.ts", { startLine: 1, endLine: 20 });
await workdir.patchLines("src/index.ts", {
  startLine: 1,
  replacement: "console.log('patched');",
});

const summary = await workdir.summarize();
```

For project/workdir roots, prefer `local.workdir()` so SDK defaults protect common generated directories such as `.git`, `node_modules`, `dist`, and build caches.

```typescript
const project = local.workdir("/path/to/project", {
  name: "my-project",
  trusted: true,
  ignore: ["vendor", /^tmp\//],
});
await project.loadIgnoreFiles();

const before = await project.snapshot();
const plan = await project.previewEdits([
  {
    path: "src/index.ts",
    startLine: 1,
    endLine: 1,
    replacement: "console.log('patched');",
  },
]);
await project.applyEdits(plan.edits);
const after = await project.snapshot();
const diff = project.diff(before, after);

const sensitivity = project.classifyPath(".env");
```

Use `createLocalContextPackage()` when a local app needs to prepare bounded workdir context for an agent request. The package includes a manifest, selected file previews, optional search matches, hashes, sensitivity labels, and secret-aware omission by default.

```typescript
import { createLocalContextPackage } from "@agent-api/sdk/local";

const context = await createLocalContextPackage(project, {
  query: "billing",
  includeSearch: true,
  maxFiles: 80,
  maxBytes: 256 * 1024,
});
```

## Production features

- **Retries:** automatic exponential backoff for network failures, 429, and 5xx (default 2 retries).
- **Timeouts:** 10 minute default for requests; 1 hour for streaming agent runs (configurable via `timeout` / `streamTimeout`).
- **Cancellation:** pass `signal` in request options to abort local HTTP waiting, and call `client.responses.cancel(responseID)` for backend best-effort cancellation after a response ID exists.
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
}, { signal: abortController.signal });

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
