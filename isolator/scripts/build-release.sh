#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Usage: scripts/build-release.sh <version>" >&2
  echo "Example: scripts/build-release.sh 1.3.0" >&2
  exit 2
fi

case "$VERSION" in
  v*) TAG_VERSION="$VERSION" ;;
  *) TAG_VERSION="v$VERSION" ;;
esac

OUT_DIR="$ROOT/dist/$TAG_VERSION"
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

build() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  if [ "$goos" = "windows" ]; then
    ext=".exe"
  fi
  local name="agent-isolator-${TAG_VERSION}-${goos}-${goarch}${ext}"
  echo "building $name"
  (
    cd "$ROOT"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
      -trimpath \
      -ldflags "-s -w -X main.version=$TAG_VERSION" \
      -o "$OUT_DIR/$name" \
      ./cmd/agent-isolator
  )
}

build linux amd64
build linux arm64
build darwin amd64
build darwin arm64
build windows amd64
build windows arm64

(
  cd "$OUT_DIR"
  sha256sum * > checksums.txt
)

cat > "$OUT_DIR/README.txt" <<EOF
agent-isolator $TAG_VERSION

Install:
1. Download the artifact for your OS/architecture.
2. Rename it to agent-isolator
   - Windows: agent-isolator.exe
   - macOS/Linux: agent-isolator
3. Put it on PATH.
4. Verify:
   echo '{"id":"status","method":"status","params":{}}' | agent-isolator --once --driver=auto

For a full native smoke test from source, run:

AGENT_ISOLATOR_NATIVE_SMOKE=1 go test ./cmd/agent-isolator -run TestAgentIsolatorNativeSmoke -v
EOF

cat > "$OUT_DIR/RELEASE_NOTES.md" <<'EOF'
## agent-isolator __TAG_VERSION__

Prebuilt `agent-isolator` binaries for Agent API local shell isolation.

### Artifacts

- `agent-isolator-__TAG_VERSION__-linux-amd64`
- `agent-isolator-__TAG_VERSION__-linux-arm64`
- `agent-isolator-__TAG_VERSION__-darwin-amd64`
- `agent-isolator-__TAG_VERSION__-darwin-arm64`
- `agent-isolator-__TAG_VERSION__-windows-amd64.exe`
- `agent-isolator-__TAG_VERSION__-windows-arm64.exe`
- `checksums.txt`

### Install

Download the artifact for your OS/architecture, rename it to
`agent-isolator` or `agent-isolator.exe`, put it on `PATH`, and verify:

```bash
echo '{"id":"status","method":"status","params":{}}' | agent-isolator --once --driver=auto
```

### SDK Behavior

The JS, Python, and Go SDKs do not require this binary for direct local shell
execution. When configured with isolation auto mode, SDKs try `agent-isolator`
first and fall back to direct execution if it is unavailable. Required mode
fails closed when no isolating runner can be selected.
EOF
sed -i.bak "s/__TAG_VERSION__/$TAG_VERSION/g" "$OUT_DIR/RELEASE_NOTES.md"
rm -f "$OUT_DIR/RELEASE_NOTES.md.bak"

echo
echo "Artifacts written to $OUT_DIR"
cat "$OUT_DIR/checksums.txt"
