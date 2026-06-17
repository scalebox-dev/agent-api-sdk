# Python SDK

Production Python SDK for the Managed Agent API.

**Published on PyPI:** [`cloudsway-agent`](https://pypi.org/project/cloudsway-agent/) (v1.1.0)

## Install

```bash
pip install cloudsway-agent
```

For local development from this repository:

```bash
cd sdk/python
pip install -e .
```

## Quick start

```python
from agent_api import AgentAPI

client = AgentAPI(
    api_key="sk-...",
    base_url="https://api.agentsway.dev",
)

response = client.responses.create(
    preset="pro-search",
    input="What changed in AI this week?",
)
print(response["output_text"])
client.close()
```

Environment variables `AGENT_API_KEY` and `AGENT_API_BASE_URL` are used by default.
The default base URL is `https://api.agentsway.dev` when neither argument nor env is set.
`AsyncAgentAPI` is available for async integrations.

## Package layout

```
src/agent_api/
  client.py            # synchronous AgentAPI
  async_client.py      # AsyncAgentAPI
  errors.py            # typed exceptions
  pagination.py          # cursor pagination
  streaming.py           # SSE parser
  _http.py               # retries, timeouts, User-Agent
  local/                 # local runtime/workspace support
  resources/             # responses, models, presets, tools, volumes, skills
  types/                 # TypedDict contracts
```

## Resources

| Resource | Methods |
|----------|---------|
| `client.responses` / `client.agent` | `create`, `list`, `list_page`, `list_iterator`, `retrieve`, `cancel`, `list_children`, `list_events` |
| `client.models` | `list` |
| `client.presets` | `list` |
| `client.tools` | `list` |
| `client.volumes` | `list`, `create`, `retrieve`, `update`, `delete`, `list_entries`, `search_entries`, `read_file`, `write_file`, `delete_path`, `reconcile_usage`, `create_directory`, `download_archive`, `summarize`, `read_lines`, `patch_lines`, `grep` |
| `client.skills` | `list`, `create`, `discover`, `focus`, `create_dev`, `update_file`, `retrieve`, `update`, `archive`, `delete`, `diff`, `accept_dev`, `discard_dev`, `export_archive`, `import_archive`, `push_directory`, `pull_directory`, `list_files`, `read_file`, `write_file`, `delete_file` |

## Durable Volumes

```python
volume = client.volumes.create(name="research-notes")

client.volumes.write_file(volume["volume_id"], "notes/summary.md", "# Summary\n")
file = client.volumes.read_file(volume["volume_id"], "notes/summary.md")
binary = client.volumes.read_file(volume["volume_id"], "assets/logo.png", format="raw")

response = client.agent.create(
    preset="pro-search",
    input="Use the attached workspace volume.",
    volume_id=volume["volume_id"],
)
```

## Skills

`local_skill_from_directory()` reads `SKILL.md` into the descriptor for initial local-skill auto-focus; later focused reads still use the local skill tool bridge.

```python
from agent_api import local_skill_from_directory

skill = client.skills.create(name="research-helper")
client.skills.write_file(skill["skill_id"], "SKILL.md", "# Research helper\n")

local_skill = local_skill_from_directory("./skills/research-helper")
response = client.responses.create(
    input="Use the research helper.",
    skills=[{"skill_id": skill["skill_id"], "branch": "main"}],
    local_skills=[local_skill],
)
```

## Local Runtime

Local app and CLI integrations can use `agent_api.local` for framework-neutral filesystem and workspace support. It is not a desktop UI kit; Electron, Qt, Tauri, or native apps should keep UI policy in their host framework and call this layer from a trusted local process.

```python
from agent_api.local import create_local_context_package, create_local_runtime

local = create_local_runtime(app_name="agent-studio")
local.ensure()

local.config.set("settings.json", "baseURL", "https://api.agentsway.dev")
local.cache.write_json("models.json", [{"id": "openai/gpt-5.5"}])

project = local.workspace("/path/to/project", name="my-project", trusted=True)
project.load_ignore_files()

matches = project.grep(pattern="billing", path="src")
summary = project.summarize()
before = project.snapshot()

plan = project.preview_edits([
    {
        "path": "src/app.py",
        "start_line": 1,
        "end_line": 1,
        "replacement": "print('patched')",
    }
])
project.apply_edits(plan["edits"])
after = project.snapshot()
diff = project.diff(before, after)

context = create_local_context_package(
    project,
    query="billing",
    include_search=True,
    max_files=80,
    max_bytes=256 * 1024,
)
```

The local runtime provides cross-platform app directories, root-scoped file stores, atomic text/JSON/byte writes, workbench-style entry search and file delivery, line edits, grep, summaries, default workspace ignore rules, `.gitignore` loading, snapshots, diffs, conflict-aware multi-file edits with rollback, local skill discovery, sensitivity classification, and bounded context packages for agent handoff.

## Production features

- **Retries:** exponential backoff for network failures, 429, and 5xx (default 2 retries).
- **Timeouts:** 10 minute default; 1 hour for streaming (override with `timeout` / `stream_timeout`).
- **Typed errors:** `AuthenticationError`, `RateLimitError`, `NotFoundError`, etc.
- **Pagination:** `list_page()` and `list_iterator()` for cursor-based history.

```python
for item in client.responses.list_iterator(limit=20):
    print(item["id"], item["status"])
```

## Model routing

```python
response = client.responses.create(
    input="Compare two cloud providers for ML workloads.",
    model_routing="auto",
    routing_strategy="cost-effective",
    models=["openai/gpt-5.4", "google/gemini-3-flash-preview"],
)
```

Use model ids in vendor/model form (values from `client.models.list()`). Omit `model_routing` (or set `"chain"`) for strict fallback order via `model` / `models`. `routing_strategy` is only valid when `model_routing` is `"auto"`.

## Streaming

```python
for event in client.responses.create(
    preset="fast-search",
    input="Summarize today's AI news.",
    stream=True,
):
    if event["type"] == "response.output_text.delta":
        print(event.get("delta", ""), end="")
```

## Client options

```python
client = AgentAPI(
    api_key="sk-...",
    base_url="https://api.agentsway.dev",
    timeout=600.0,
    stream_timeout=3600.0,
    max_retries=2,
)
```

## Tests

```bash
PYTHONPATH=src python -m unittest discover -s tests -p 'test_agent_api.py' -v
AGENT_API_INTEGRATION=1 AGENT_API_KEY=sk-... AGENT_API_BASE_URL=https://api.agentsway.dev \
  PYTHONPATH=src python -m unittest tests.test_integration -v
```

## Scope

The SDK covers the public agent/Responses API, durable volume APIs, skill APIs, and discovery
endpoints. Console auth, workspace administration, and internal audit records
are intentionally out of scope.
