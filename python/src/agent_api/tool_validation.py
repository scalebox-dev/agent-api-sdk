from __future__ import annotations

from collections.abc import Mapping, Sequence
from typing import Any


def validate_unique_tool_names(tools: Sequence[Mapping[str, Any]] | None) -> None:
    if not tools:
        return
    seen: set[str] = set()
    for tool in tools:
        name = str(tool.get("name", "")).strip()
        if not name:
            raise ValueError("tools[].name is required")
        if name in seen:
            raise ValueError(f"duplicate tools[].name: {name}")
        seen.add(name)
