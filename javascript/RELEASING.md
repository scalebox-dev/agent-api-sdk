# Releasing @agent-api/sdk

This package is **independent** from the Python `cloudsway-agent` package. Versions and
release tags are not coupled.

## Bump and release

1. Set version in `package.json`.
2. Sync generated constants and update `CHANGELOG.md`:

```bash
cd javascript
npm install
npm run sync-version
npm test
```

3. Commit, tag, and push:

```bash
git add package.json package-lock.json src/version.ts CHANGELOG.md
git commit -m "chore(sdk/js): release v1.0.1"
git tag javascript/v1.0.1
git push origin HEAD --tags
```

Tag `javascript/v*` triggers [`.github/workflows/sdk-javascript-release.yml`](../.github/workflows/sdk-javascript-release.yml) when tag triggers are enabled.

## Secrets

| Secret | Purpose |
|--------|---------|
| `NPM_TOKEN` | npm publish for `@agent-api/sdk` |

Local credential files (`.npm.env`) are gitignored and **not** included in the npm
package: `package.json` `"files"` whitelists only `dist/`, docs, and `LICENSE`. CI
rejects any release tarball that lists env credential paths.

## Manual release

**Actions → SDK JavaScript Release → Run workflow**
