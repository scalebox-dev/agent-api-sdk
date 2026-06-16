from __future__ import annotations

from typing import Any, Literal, TypedDict

AgentCapabilityPreference = Literal["off", "auto", "preferred", "required"]
ResponseStatus = Literal["completed", "failed", "in_progress", "cancelled", "requires_action"]
OutputItemStatus = Literal["completed", "failed", "in_progress"]

JSONValue = str | int | float | bool | None | list["JSONValue"] | dict[str, "JSONValue"]


class ErrorInfo(TypedDict, total=False):
    message: str
    type: str
    code: str


class Usage(TypedDict, total=False):
    input_tokens: int
    output_tokens: int
    total_tokens: int
    input_tokens_details: dict[str, int]
    output_tokens_details: dict[str, int]
    tool_calls_details: dict[str, dict[str, int]]
    cost: dict[str, float | str]


class ClientOptions(TypedDict, total=False):
    api_key: str
    base_url: str
    timeout: float
    stream_timeout: float
    max_retries: int
    default_headers: dict[str, str]
