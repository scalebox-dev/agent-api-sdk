from __future__ import annotations

import json
from collections.abc import Awaitable, Callable, Mapping
from typing import Any

from agent_api.types import AgentResponse, FunctionCallOutputInput

LocalFunctionHandler = Callable[[dict[str, Any]], str | dict[str, Any] | Awaitable[str | dict[str, Any]]]


def pending_function_calls(response: AgentResponse) -> list[dict[str, Any]]:
    if response.get("status") != "requires_action":
        return []
    return [item for item in response.get("output", []) if item.get("type") == "function_call"]


def function_call_output_input(call_id: str, output: str | dict[str, Any]) -> FunctionCallOutputInput:
    text = output if isinstance(output, str) else json.dumps(output)
    return {"type": "function_call_output", "call_id": call_id, "output": text}


async def run_local_function_handlers(
    response: AgentResponse,
    handlers: Mapping[str, LocalFunctionHandler],
) -> list[FunctionCallOutputInput]:
    pending = pending_function_calls(response)
    outputs: list[FunctionCallOutputInput] = []
    for call in pending:
        name = str(call.get("name", ""))
        handler = handlers.get(name)
        if handler is None:
            raise ValueError(f"no local handler registered for function {name}")
        raw_args = call.get("arguments") or "{}"
        args = json.loads(raw_args) if isinstance(raw_args, str) else dict(raw_args)
        result = handler(args)
        if hasattr(result, "__await__"):
            result = await result  # type: ignore[misc]
        call_id = str(call.get("call_id", ""))
        outputs.append(function_call_output_input(call_id, result))  # type: ignore[arg-type]
    return outputs
