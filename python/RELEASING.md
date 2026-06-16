# Releasing cloudsway-agent

This package is **independent** from the JavaScript `@agent-api/sdk` package.
Versions and release tags are not coupled.

## Bump and release

1. Set version in `pyproject.toml`.
2. Sync generated constants and update `CHANGELOG.md`:

```bash
cd sdk/python
python3 scripts/sync_version.py
python3 -m venv .venv && .venv/bin/pip install httpx build
PYTHONPATH=src .venv/bin/python -m unittest discover -s tests -p 'test_agent_api.py' -v
.venv/bin/python -m build
```

3. Commit, tag, and push:

```bash
git add pyproject.toml src/agent_api/_version.py CHANGELOG.md
git commit -m "chore(sdk/py): release v1.0.1"
git tag sdk/python/v1.0.1
git push origin HEAD --tags
```

Tag `sdk/python/v*` triggers [`.github/workflows/sdk-python-release.yml`](../../.github/workflows/sdk-python-release.yml).

## Secrets

| Secret | Purpose |
|--------|---------|
| `PYPI_API_TOKEN` | PyPI publish for `cloudsway-agent` |

Local credential files (`.pypi.env`) are gitignored and **not** included in PyPI
artifacts: wheels ship only `src/agent_api/`; `MANIFEST.in` excludes `.pypi.env` from
sdists. CI rejects builds that list env credential paths.

## Manual release

**Actions → SDK Python Release → Run workflow**
