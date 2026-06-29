# Changelog — @agent-api/sdk

## 1.4.0

### Added

- Added `user_id` filters to response, volume, and skill list helpers.
- Added response-only `safety_identifier` filtering and retrieve guards.
- Added `user_id` and `safety_identifier` fields to response and response list item types.

## 1.3.2

### Added

- Added volume image asset helpers for normalizing private volume paths and checking supported image paths/content types.

## 1.3.1

### Fixed

- Made `local_workdir` grep behave like familiar grep: `path` may be omitted, a file path, or a directory subtree.
- Model-facing `local_workdir` execution errors now return structured `{ ok: false, error }` tool results instead of aborting agent runs.

## 1.3.0

### Added

- Added SDK support for `local_shell` isolation modes: `none`, `auto`, and `required`.
- Added `IsolatorLocalShellRunner`, a Node-local runner that delegates shell execution to the shared `agent-isolator` binary.
- Added model-facing `shell_isolation` metadata for approval previews and execution results.

### Changed

- `local_shell` auto isolation now tries an explicitly configured `agent-isolator` path first and falls back to direct host execution when isolation is unavailable.
- SDKs no longer assume `agent-isolator` is discoverable on `PATH`; pass `isolator.executablePath` or set `AGENT_ISOLATOR_PATH`.
- Direct host execution remains the default/fallback path and reports its actual non-isolated guarantees.

## 1.2.3

### Fixed

- Hardened local workdir scans so broken symlinks and files that disappear during recursive list, grep, and summarize operations do not abort the scan.

## 1.2.2

### Added

- Added `RequestOptions.signal` so CLI, TUI, and desktop integrations can abort in-flight HTTP requests and response streams before a response ID is available.
- Documented and tested response cancellation behavior alongside the existing `responses.cancel(responseID)` backend cancellation API.

## 1.2.1

### Added

- Added a model-facing `local_shell` tool primitive with approval/full-access modes, bounded host command execution, timeout handling, execution-environment context, and a pluggable command-runner boundary for future isolation backends.

## 1.2.0

### Added

- Added browser-session refresh helpers for long-running CLI and desktop integrations: `client.auth.refreshBrowserSession()` and `browserAuthSessionExpiresWithin()`.
- Added unified local skill tool-call handling for both local skill and function-call response items.

### Changed

- Renamed local SDK concepts from workspace to workdir to avoid confusion with platform identity workspaces.
- Extended response input types so tool-call continuations can carry function-call metadata consistently.

## 1.1.2

### Added

- Added a model-facing `local_workdir` driver/tool primitive for local workdir operations.
- Added approval-aware dispatch for mutating local workdir actions.

### Changed

- Replaced fragmented local workdir tool presentation with an action-based adapter over the low-level local APIs.

## 1.1.1

### Added

- Browser/device login helpers for CLI and desktop integrations: `client.auth.startDeviceAuth()`, `client.auth.pollDeviceAuth()`, and `client.auth.waitForDeviceAuth()`.
- Convenience methods on `AgentAPI`: `startDeviceAuth()`, `pollDeviceAuth()`, and `waitForDeviceAuth()`.
- `DeviceAuthFlowError` for expired, consumed, or timed-out device login flows.

## 1.1.0

### Added

- Node-only `@agent-api/sdk/local` entrypoint for framework-neutral local app and CLI runtime support.
- Cross-platform app directory resolution for data, config, cache, logs, and temp files.
- Root-scoped local file stores with path traversal protection, atomic writes, JSON helpers, recursive listing, and local skill discovery.
- Local workdir operations for entry search, file delivery, line-range reads and edits, literal grep, and directory summaries.
- `LocalWorkdir` and `LocalWorkdirManager` for project roots, default ignore rules, patch previews, snapshots, diffs, and file-watch handles.
- Typed local errors, `.gitignore` loading, sensitivity classification for likely secret paths, and multi-file line-edit plans with conflict detection and rollback.
- `createLocalContextPackage()` for bounded, secret-aware local workdir manifests that app integrations can hand to agent workflows.

### Changed

- Split the local SDK source behind a small `@agent-api/sdk/local` barrel so runtime and context APIs can grow without a monolithic entrypoint.

## 1.0.8

### Changed

- Expanded `reasoning.effort` types to match the Responses-compatible enum: `none`, `minimal`, `low`, `medium`, `high`, and `xhigh`.

## 1.0.7

### Added

- Skill artifact archive APIs: `client.skills.exportArchive()` and `client.skills.importArchive()`.
- Local directory sync helpers for platform skills: `client.skills.pushDirectory()` and `client.skills.pullDirectory()`.
- Skill branch diff API and file-level `acceptDev()` strategies (`patch` or `mirror`).

## 1.0.6

### Changed

- Replaced skill focus `max_manifest_bytes` with `max_manifest_chars`.
- Local skill `SKILL.md` manifests are decoded as strict UTF-8 and truncated by characters.

## 1.0.5

### Added

- Main/dev skill branch support and model-facing skill operations for progressive discover, focus, and update workflows.

## 1.0.4

### Added

- `preferred_sites` create-run parameter (up to 3 hostnames) to bias web search and fetch toward specific domains when allowed tools include `web_search` or `web_fetch`.
- Skill APIs via `client.skills`, create-run `skills` / `local_skills` parameters, and `localSkillFromDirectory()` for SDK-local skill descriptors.

## 1.0.3

### Added

- Workbench volume APIs: `summarize`, `grep`, `readLines`, `patchLines`, `downloadArchive`.
- `responses.retrieveVolume` for `GET /v1/responses/{response_id}/volume`.
- Create-run params `model_routing` (`auto` | `chain`) and `routing_strategy` (`balanced` | `high-quality` | `cost-effective`) for platform model selection when `model_routing` is `auto`.

### Changed

- **Breaking:** `readFile` returns smart delivery metadata (`encoding`, `mime_type`, `content`, `image_url`, etc.); use `{ format: "raw" }` for binary bytes.
- **Breaking:** `deleteFile` and `deleteDirectory` replaced by unified `deletePath`, returning `{ path, recursive }`.

## 1.0.2

### Added

- `user` on create-run params; `input_document` content parts in TypeScript types.
- `response.requires_action` in streaming event types.
- `readFile(..., { format: "raw" })` for binary volume files (`VolumeFileRaw` with `ArrayBuffer` content).
- Route manifest coverage for volume rename, usage reconcile, and directory create/delete.

### Fixed

- README and route manifest now list all volume resource methods implemented since 1.0.1.

## 1.0.1

### Added

- Durable volume resource methods for volume CRUD, entry listing/search, and file read/write/delete operations.
- `volume_id` create-run parameter for attaching an existing durable volume to agent runs.

## 1.0.0

### Added

- Modular package layout with retries, typed errors, pagination, and streaming.
- CI via `.github/workflows/sdk.yml` and npm publish via `sdk-javascript-release.yml`.
- Route coverage check via `scripts/check-routes.mjs`.

### Notes

- Independent from the Python `cloudsway-agent` package; version and release tags are separate.
- Registry: https://www.npmjs.com/package/@agent-api/sdk
