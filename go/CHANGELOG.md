# Changelog - Go SDK

## 1.1.0

### Added

- `agentapi/local` package for framework-neutral local app and CLI runtime support.
- Cross-platform app directory resolution for data, config, cache, logs, and temp files.
- Root-scoped local file stores with path traversal protection, atomic writes, JSON helpers, recursive listing, and local skill discovery.
- Local workdir operations for entry search, file delivery, line-range reads and edits, literal grep, and directory summaries.
- `Workspace` and `WorkspaceManager` for project roots, default ignore rules, `.gitignore` loading, patch previews, snapshots, diffs, and conflict-aware multi-file edit plans with rollback.
- Sensitivity classification for likely secret paths and `CreateContextPackage` for bounded, secret-aware local workspace manifests.

## 1.0.0

### Added

- Initial Go SDK for the Managed Agent API.
- Responses, Agent, discovery, volume, and skill resources.
- SSE streaming, retries, typed errors, timeouts, and route coverage checks.
