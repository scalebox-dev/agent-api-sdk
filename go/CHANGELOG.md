# Changelog - Go SDK

## 1.4.0

### Added

- Added `client.SafetyIdentifiers.List` and `client.SafetyIdentifiers.Lookup` for the public safety identifier registry APIs.
- Added `SafetyIdentifier` filtering to response list helpers.
- Added `SafetyIdentifier` fields to response and response list item types.

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
- Added `IsolatorLocalShellRunner`, a local runner that delegates shell execution to the shared `agent-isolator` binary.
- Added model-facing `shell_isolation` metadata for approval previews and execution results.

### Changed

- `local_shell` auto isolation now tries an explicitly configured `agent-isolator` path first and falls back to direct host execution when isolation is unavailable.
- SDKs no longer assume `agent-isolator` is discoverable on `PATH`; pass `IsolatorOptions.ExecutablePath` or set `AGENT_ISOLATOR_PATH`.
- Direct host execution remains the default/fallback path and reports its actual non-isolated guarantees.

## 1.2.3

### Fixed

- Hardened local workdir scans so broken symlinks and files that disappear during recursive list, grep, and summarize operations do not abort the scan.

## 1.2.2

### Added

- Documented response cancellation behavior for local apps and CLIs. Go integrations can cancel in-flight SDK requests through `context.Context` and can call `client.Responses.Cancel(ctx, responseID)` for backend best-effort cancellation.

## 1.2.1

### Added

- Added a model-facing `local_shell` tool primitive with approval/full-access modes, bounded host command execution, timeout handling, execution-environment context, and a pluggable command-runner boundary for future isolation backends.

## 1.2.0

### Added

- Added browser-session refresh helpers for long-running CLI and desktop integrations: `client.Auth.RefreshBrowserSession`, `client.RefreshBrowserSession`, and `BrowserAuthSessionExpiresWithin`.

### Changed

- Renamed local SDK concepts from workspace to workdir to avoid confusion with platform identity workspaces.

## 1.1.1

### Added

- Browser/device login helpers for CLI and desktop integrations via `client.Auth.StartDeviceAuth`, `client.Auth.PollDeviceAuth`, and `client.Auth.WaitForDeviceAuth`.
- Convenience methods on `Client`: `StartDeviceAuth`, `PollDeviceAuth`, and `WaitForDeviceAuth`.
- `DeviceAuthFlowError` for expired, consumed, or timed-out device login flows.

## 1.1.0

### Added

- `agentapi/local` package for framework-neutral local app and CLI runtime support.
- Cross-platform app directory resolution for data, config, cache, logs, and temp files.
- Root-scoped local file stores with path traversal protection, atomic writes, JSON helpers, recursive listing, and local skill discovery.
- Local workdir operations for entry search, file delivery, line-range reads and edits, literal grep, and directory summaries.
- `Workdir` and `WorkdirManager` for project roots, default ignore rules, `.gitignore` loading, patch previews, snapshots, diffs, and conflict-aware multi-file edit plans with rollback.
- Sensitivity classification for likely secret paths and `CreateContextPackage` for bounded, secret-aware local workdir manifests.

## 1.0.0

### Added

- Initial Go SDK for the Managed Agent API.
- Responses, Agent, discovery, volume, and skill resources.
- SSE streaming, retries, typed errors, timeouts, and route coverage checks.
