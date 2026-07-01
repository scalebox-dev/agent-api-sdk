from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any, Mapping, cast

from agent_api.types.tools import FunctionTool

DEFAULT_LOCAL_PAUSE_MAX_DURATION_MS = 5 * 60 * 1000


@dataclass(frozen=True)
class LocalPauseRequest:
    duration_ms: int
    reason: str | None = None


@dataclass(frozen=True)
class LocalPauseToolRegistry:
    max_duration_ms: int = DEFAULT_LOCAL_PAUSE_MAX_DURATION_MS
    tool_name: str = "local_pause"

    def definitions(self) -> list[FunctionTool]:
        return [local_pause_tool_definition(self.tool_name, max_duration_ms=self.max_duration_ms)]

    def handlers(self) -> dict[str, Any]:
        return {self.tool_name: lambda args: self.execute(self.tool_name, args)}

    def execute(self, name: str, args: Mapping[str, Any]) -> dict[str, Any]:
        if name != self.tool_name:
            raise ValueError(f"unknown local pause tool: {name}")
        request = _pause_request(args, self.max_duration_ms)
        started = time.monotonic()
        time.sleep(request.duration_ms / 1000)
        elapsed_ms = max(0, int((time.monotonic() - started) * 1000))
        return {
            "ok": True,
            "tool": self.tool_name,
            "action": "pause",
            "requested_ms": request.duration_ms,
            "elapsed_ms": elapsed_ms,
            "status": "completed",
            **({"reason": request.reason} if request.reason else {}),
        }

    def requires_approval(self, name: str, args: Mapping[str, Any] | None = None) -> bool:
        return False


def create_local_pause_tool_registry(
    *,
    max_duration_ms: int = DEFAULT_LOCAL_PAUSE_MAX_DURATION_MS,
    tool_name: str = "local_pause",
) -> LocalPauseToolRegistry:
    return LocalPauseToolRegistry(max_duration_ms=max(1, int(max_duration_ms)), tool_name=tool_name)


def local_pause_tool_definition(name: str = "local_pause", *, max_duration_ms: int = DEFAULT_LOCAL_PAUSE_MAX_DURATION_MS) -> FunctionTool:
    return {
        "type": "function",
        "name": name,
        "description": local_pause_tool_instructions(max_duration_ms=max_duration_ms),
        "parameters": _local_pause_tool_parameters(max_duration_ms=max_duration_ms),
        "strict": False,
    }


def local_pause_tool_instructions(*, max_duration_ms: int = DEFAULT_LOCAL_PAUSE_MAX_DURATION_MS) -> str:
    max_duration_ms = max(1, int(max_duration_ms))
    return " ".join(
        [
            "Pause the local agentic workflow for a bounded amount of time, then continue automatically.",
            "Use this only when waiting for external state such as CI, deployment rollout, rate-limit cooldown, file sync, or another asynchronous process.",
            "The pause is local runtime control, not reasoning time. Keep the reason concrete.",
            f"Maximum duration: {max_duration_ms} ms.",
        ]
    )


def _local_pause_tool_parameters(*, max_duration_ms: int) -> dict[str, Any]:
    max_duration_ms = max(1, int(max_duration_ms))
    return {
        "type": "object",
        "properties": {
            "duration_ms": {
                "type": "integer",
                "minimum": 1,
                "maximum": max_duration_ms,
                "description": "How long to wait before continuing automatically, in milliseconds.",
            },
            "reason": {
                "type": "string",
                "description": "Short reason for the wait, such as the external state being awaited.",
            },
        },
        "required": ["duration_ms"],
        "additionalProperties": False,
    }


def _pause_request(args: Mapping[str, Any], max_duration_ms: int) -> LocalPauseRequest:
    raw = args.get("duration_ms", args.get("durationMs"))
    if not isinstance(raw, (int, float)):
        raise ValueError("local_pause duration_ms must be a finite number")
    duration_ms = int(raw)
    if duration_ms < 1:
        raise ValueError("local_pause duration_ms must be at least 1")
    if duration_ms > max_duration_ms:
        raise ValueError(f"local_pause duration_ms must be <= {max_duration_ms}")
    reason = args.get("reason")
    return LocalPauseRequest(
        duration_ms=duration_ms,
        reason=cast(str, reason).strip() if isinstance(reason, str) and reason.strip() else None,
    )
