from __future__ import annotations

import asyncio
import json

from agent_api.local_functions import function_call_output_input, pending_function_calls, run_local_function_handlers


def test_pending_function_calls_empty_when_completed() -> None:
    assert pending_function_calls({"id": "r1", "object": "response", "status": "completed", "model": "m", "output": []}) == []


def test_pending_function_calls_from_requires_action() -> None:
    response = {
        "id": "r1",
        "object": "response",
        "status": "requires_action",
        "model": "m",
        "output": [
            {"type": "function_call", "id": "fc_1", "status": "in_progress", "name": "get_weather", "call_id": "call_1", "arguments": '{"city":"Boston"}'}
        ],
    }
    pending = pending_function_calls(response)
    assert len(pending) == 1
    assert pending[0]["name"] == "get_weather"


def test_function_call_output_input() -> None:
    item = function_call_output_input("call_1", {"temp_f": 72})
    assert item["type"] == "function_call_output"
    assert item["call_id"] == "call_1"
    assert json.loads(item["output"]) == {"temp_f": 72}


def test_run_local_function_handlers() -> None:
    async def run() -> None:
        response = {
            "id": "r1",
            "object": "response",
            "status": "requires_action",
            "model": "m",
            "output": [
                {"type": "function_call", "id": "fc_1", "status": "in_progress", "name": "get_weather", "call_id": "call_1", "arguments": '{"city":"Boston"}'}
            ],
        }

        def get_weather(args: dict[str, object]) -> str:
            assert args["city"] == "Boston"
            return "72F"

        outputs = await run_local_function_handlers(response, {"get_weather": get_weather})
        assert len(outputs) == 1
        assert outputs[0]["output"] == "72F"

    asyncio.run(run())
