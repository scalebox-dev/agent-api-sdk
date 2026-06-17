# Changelog — cloudsway-agent

## 1.1.0

### Added

- `agent_api.local` package for framework-neutral local app and CLI runtime support.
- Cross-platform app directory resolution for data, config, cache, logs, and temp files.
- Root-scoped local file stores with path traversal protection, atomic writes, JSON helpers, recursive listing, and local skill discovery.
- Local workdir operations for entry search, file delivery, line-range reads and edits, literal grep, and directory summaries.
- `LocalWorkspace` and `LocalWorkspaceManager` for project roots, default ignore rules, `.gitignore` loading, patch previews, snapshots, diffs, and conflict-aware multi-file edit plans with rollback.
- Sensitivity classification for likely secret paths and `create_local_context_package()` for bounded, secret-aware local workspace manifests.

### Changed

- Added `pytest` to the Python SDK dev extras because the repo test suite uses pytest.

## 1.0.8

### Changed

- Expanded `reasoning.effort` TypedDict support to match the Responses-compatible enum: `none`, `minimal`, `low`, `medium`, `high`, and `xhigh`.

## 1.0.7

### Added

- Skill artifact archive APIs: `client.skills.export_archive()` and `client.skills.import_archive()`.
- Local directory sync helpers for platform skills: `client.skills.push_directory()` and `client.skills.pull_directory()`.
- Skill branch diff API and file-level `accept_dev()` strategies (`patch` or `mirror`).

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
- Skill APIs via `client.skills`, create-run `skills` / `local_skills` parameters, and `local_skill_from_directory()` for SDK-local skill descriptors.

## 1.0.3

### Added

- Workbench volume APIs: `summarize`, `grep`, `read_lines`, `patch_lines`, `download_archive`.
- `responses.retrieve_volume` for `GET /v1/responses/{response_id}/volume`.

### Changed

- **Breaking:** `read_file` returns smart delivery metadata (`encoding`, `mime_type`, `content`, `image_url`, etc.) via `DeliverFile`; use `format="raw"` for binary bytes.
- **Breaking:** `delete_file` and `delete_directory` replaced by unified `delete_path` (`DELETE /v1/volumes/{id}/paths/{path}`), returning `{path, recursive}`.

## 1.0.2

### Added

- `user` on create-run params; `input_document` content parts in TypedDicts.
- `response.requires_action` in streaming event types.
- `read_file(..., format="raw")` for binary volume files (`VolumeFileRaw` with `bytes` content).
- `AsyncAgentAPI.responses.list_page` and `list_iterator` for async cursor pagination.
- Route manifest coverage for volume rename, usage reconcile, and directory create/delete.

### Fixed

- README and route manifest now list all volume resource methods implemented since 1.0.1.

## 1.0.1

### Added

- Durable volume resource methods for volume CRUD, entry listing/search, and file read/write/delete operations.
- `volume_id` create-run parameter for attaching an existing durable volume to agent runs.

## 1.0.0

### Added

- Modular package layout with sync/async clients, retries, typed errors, pagination, and streaming.
- CI via `.github/workflows/sdk.yml` and PyPI publish via `sdk-python-release.yml`.
- Route coverage check via `scripts/check_routes.py`.

### Notes

- Independent from the JavaScript `@agent-api/sdk` package; version and release tags are separate.
- Published on PyPI as `cloudsway-agent`; import with `from agent_api import AgentAPI`.
- Registry: https://pypi.org/project/cloudsway-agent/
