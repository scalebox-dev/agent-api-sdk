#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
PYPROJECT = ROOT / "pyproject.toml"
VERSION_PY = ROOT / "src" / "agent_api" / "_version.py"

match = re.search(r'^version = "(.+)"$', PYPROJECT.read_text(), re.MULTILINE)
if not match:
    print("Could not read version from pyproject.toml", file=sys.stderr)
    sys.exit(1)

version = match.group(1)
VERSION_PY.write_text(
    f'''"""Agent API Python SDK version."""

__version__ = "{version}"
USER_AGENT = f"cloudsway-agent/{{__version__}}"

DEFAULT_TIMEOUT = 600.0
DEFAULT_STREAM_TIMEOUT = 3600.0
DEFAULT_MAX_RETRIES = 2
'''
)
print(f"Synced cloudsway-agent version {version} -> src/agent_api/_version.py")
