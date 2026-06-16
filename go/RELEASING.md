# Releasing the Go SDK

The Go SDK is a module under the public `agent-api-sdk` repository:

```text
github.com/scalebox-dev/agent-api-sdk/go
```

## Release

1. Update `CHANGELOG.md`.
2. Run checks:

```bash
cd go
go test ./...
go run ./scripts/check_routes.go
```

3. Commit and tag from the repository root:

```bash
git add go
git commit -m "chore(sdk/go): release v1.0.0"
git tag go/v1.0.0
git push origin HEAD --tags
```

Go modules are resolved from git tags. No registry publish step is required.
