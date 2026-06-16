from __future__ import annotations

from typing import Any

from agent_api.types import AgentResponse


def drop_none(values: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in values.items() if value is not None}


def add_output_text(response: AgentResponse) -> AgentResponse:
    if "output_text" in response:
        return response
    text = ""
    for item in response.get("output", []):
        if item.get("type") != "message":
            continue
        for part in item.get("content", []):
            if part.get("type") == "output_text":
                text += part.get("text", "")
    response["output_text"] = text
    return response


def build_query(params: dict[str, Any]) -> str:
    filtered = {key: value for key, value in params.items() if value is not None and value != ""}
    if not filtered:
        return ""
    from urllib.parse import urlencode

    return "?" + urlencode(filtered)
