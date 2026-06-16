#!/usr/bin/env bash
# Local npm publish helper for @agent-api/sdk (not shipped in the package).
#
# Auth (either is fine):
#   - `npm login` in this environment (interactive), or
#   - sdk/javascript/.npm.env with NPM_TOKEN=... (gitignored)
#
# Usage:
#   ./local_publish.sh           # test, verify tarball, publish
#   ./local_publish.sh --dry-run # test and verify only; no publish

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
elif [[ -n "${1:-}" ]]; then
  echo "usage: $0 [--dry-run]" >&2
  exit 2
fi

if [[ -f .npm.env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .npm.env
  set +a
fi

if [[ -n "${NPM_TOKEN:-}" ]]; then
  export NODE_AUTH_TOKEN="$NPM_TOKEN"
fi

VERSION="$(node -p "require('./package.json').version")"
echo "Preparing @agent-api/sdk@${VERSION} for npm publish"

npm ci
npm run sync-version
npm run check:routes
npm test

echo "Checking publish tarball excludes credential env files..."
npm pack --dry-run 2>&1 | tee /tmp/npm-pack-local.txt
if grep -E '\.npm\.env|\.pypi\.env|(^|/)\.env(\.|$)' /tmp/npm-pack-local.txt; then
  echo "Credential env file would be published to npm — aborting." >&2
  exit 1
fi

if [[ "$DRY_RUN" == true ]]; then
  echo "Dry run complete (tests passed; tarball looks safe). Skipping npm publish."
  exit 0
fi

if [[ -z "${NODE_AUTH_TOKEN:-}" ]] && ! npm whoami >/dev/null 2>&1; then
  echo "Not logged in to npm and NPM_TOKEN is unset. Run 'npm login' or add NPM_TOKEN to .npm.env." >&2
  exit 1
fi

npm publish --access public
echo "Published @agent-api/sdk@${VERSION} to https://www.npmjs.com/package/@agent-api/sdk/v/${VERSION}"
