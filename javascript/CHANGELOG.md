# Changelog — @agent-api/sdk

## 1.1.0

### Added

- Node-only `@agent-api/sdk/local` entrypoint for framework-neutral local app and CLI runtime support.
- Cross-platform app directory resolution for data, config, cache, logs, and temp files.
- Root-scoped local file stores with path traversal protection, atomic writes, JSON helpers, recursive listing, and local skill discovery.
- Local workdir operations for entry search, file delivery, line-range reads and edits, literal grep, and directory summaries.
- `LocalWorkspace` and `LocalWorkspaceManager` for project roots, default ignore rules, patch previews, snapshots, diffs, and file-watch handles.
- Typed local errors, `.gitignore` loading, sensitivity classification for likely secret paths, and multi-file line-edit plans with conflict detection and rollback.

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
