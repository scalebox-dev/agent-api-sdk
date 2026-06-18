# Changelog - Go SDK

## 1.1.5

### Added

- Added browser-session refresh helpers for long-running CLI and desktop integrations: `client.Auth.RefreshBrowserSession`, `client.RefreshBrowserSession`, and `BrowserAuthSessionExpiresWithin`.

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
- `Workspace` and `WorkspaceManager` for project roots, default ignore rules, `.gitignore` loading, patch previews, snapshots, diffs, and conflict-aware multi-file edit plans with rollback.
- Sensitivity classification for likely secret paths and `CreateContextPackage` for bounded, secret-aware local workspace manifests.

## 1.0.0

### Added

- Initial Go SDK for the Managed Agent API.
- Responses, Agent, discovery, volume, and skill resources.
- SSE streaming, retries, typed errors, timeouts, and route coverage checks.
