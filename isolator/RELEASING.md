# Releasing agent-isolator

`agent-isolator` is distributed as prebuilt GitHub Release artifacts. SDKs do
not build it during package installation and do not require it for direct-mode
local shell execution.

## Build Locally

```bash
cd isolator
scripts/build-release.sh 1.3.0
```

Artifacts are written to `isolator/dist/v1.3.0/`:

- `agent-isolator-v1.3.0-linux-amd64`
- `agent-isolator-v1.3.0-linux-arm64`
- `agent-isolator-v1.3.0-darwin-amd64`
- `agent-isolator-v1.3.0-darwin-arm64`
- `agent-isolator-v1.3.0-windows-amd64.exe`
- `agent-isolator-v1.3.0-windows-arm64.exe`
- `checksums.txt`
- `README.txt`
- `RELEASE_NOTES.md`

## GitHub Release

### Manual Workflow

Run **Agent Isolator Release** from GitHub Actions with:

- `version`: `1.3.0`
- `dry_run`: `true` to build/upload workflow artifacts only
- `dry_run`: `false` to create/update the GitHub Release

The workflow builds binaries, writes checksums, uploads workflow artifacts, and
publishes a GitHub Release named `agent-isolator v1.3.0` under tag
`isolator/v1.3.0`.

### Tag Trigger

When tag-triggered workflows are re-enabled, create and push the release tag:

```bash
git tag isolator/v1.3.0
git push origin isolator/v1.3.0
```

That will run the same release workflow automatically.

## Install Verification

After downloading the artifact for a target OS/architecture:

```bash
mv agent-isolator-v1.3.0-linux-amd64 agent-isolator
chmod +x agent-isolator
echo '{"id":"status","method":"status","params":{}}' | ./agent-isolator --once --driver=auto
```

For native driver validation from source:

```bash
AGENT_ISOLATOR_NATIVE_SMOKE=1 go test ./cmd/agent-isolator -run TestAgentIsolatorNativeSmoke -v
```
