#!/usr/bin/env python3
from __future__ import annotations

import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
manifest = json.loads((ROOT / "scripts" / "routes.json").read_text())
sources = {path: path.read_text() for path in (ROOT / "src" / "agent_api").rglob("*.py")}


def path_needles(operation_path: str) -> list[str]:
    import re

    static_prefix = operation_path.split("{", 1)[0].rstrip("/")
    needles = [re.sub(r"\{[^}]+\}", "", operation_path), static_prefix]
    return [needle for needle in needles if needle]


def has_path_coverage(operation_path: str, method: str) -> bool:
    needles = path_needles(operation_path)
    for content in sources.values():
        if not any(needle in content for needle in needles):
            continue
        if method == "POST" and "/cancel" in operation_path and "cancel" not in content:
            continue
        if method == "GET" and "/events" in operation_path and "events" not in content:
            continue
        if method == "GET" and "/children" in operation_path and "children" not in content:
            continue
        if method == "GET" and "/archive" in operation_path and "/archive" not in content:
            continue
        if method == "GET" and operation_path.endswith("/volume") and "/volume" not in content:
            continue
        if method == "GET" and "/grep" in operation_path and "/grep" not in content:
            continue
        if "POST" in method and "/summarize" in operation_path and "/summarize" not in content:
            continue
        if method == "GET" and "/file_lines/" in operation_path and "/file_lines/" not in content:
            continue
        if method == "PATCH" and "/file_lines/" in operation_path and "/file_lines/" not in content:
            continue
        if method == "DELETE" and "/paths/" in operation_path and "/paths/" not in content:
            continue
        return True
    return False


failures = [
    f'Missing coverage for {op["method"]} {op["path"]} ({op["symbol"]})'
    for op in manifest["operations"]
    if not has_path_coverage(op["path"], op["method"])
]

if failures:
    print("cloudsway-agent route coverage failed:", file=sys.stderr)
    for line in failures:
        print(f"- {line}", file=sys.stderr)
    sys.exit(1)

print(f"cloudsway-agent route coverage OK ({len(manifest['operations'])} operations)")
