from __future__ import annotations

import asyncio
import os
import unittest

from agent_api import AgentAPI
from agent_api.local_functions import pending_function_calls, run_local_function_handlers

SDK_LOCAL_FUNCTION_MARKER = "SDK_LOCAL_FUNCTION_MARKER"

GET_WEATHER_TOOL = {
    "type": "function",
    "name": "get_weather",
    "description": "Get weather for a city",
    "parameters": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"],
    },
}


def integration_enabled() -> bool:
    return os.environ.get("AGENT_API_INTEGRATION") == "1"


def local_function_integration_enabled() -> bool:
    return integration_enabled() and os.environ.get("AGENT_API_LOCAL_FUNCTION_TEST") == "1"


def integration_client() -> AgentAPI:
    api_key = os.environ.get("AGENT_API_KEY")
    if not api_key:
        raise unittest.SkipTest("AGENT_API_KEY is required for integration tests")
    base_url = (os.environ.get("AGENT_API_BASE_URL") or "http://127.0.0.1:18080").rstrip("/")
    return AgentAPI(api_key=api_key, base_url=base_url, timeout=180.0)


@unittest.skipUnless(
    local_function_integration_enabled(),
    "set AGENT_API_INTEGRATION=1 and AGENT_API_LOCAL_FUNCTION_TEST=1 to run local-function integration tests",
)
class LocalFunctionCallingIntegrationTest(unittest.TestCase):
    def test_pause_resume_with_local_function_handlers(self) -> None:
        client = integration_client()
        try:
            paused = client.responses.create(
                model="mock-gpt",
                input=f"{SDK_LOCAL_FUNCTION_MARKER}: What is the weather in Boston?",
                tools=[GET_WEATHER_TOOL],
                max_steps=4,
                max_output_tokens=256,
            )
            self.assertTrue(paused["id"].startswith("resp_"))
            self.assertEqual(paused["status"], "requires_action")

            pending = pending_function_calls(paused)
            self.assertEqual(len(pending), 1)
            self.assertEqual(pending[0]["name"], "get_weather")
            self.assertEqual(pending[0]["call_id"], "call_get_weather_sdk")

            outputs = asyncio.run(
                run_local_function_handlers(
                    paused,
                    {
                        "get_weather": lambda args: f"{args['city']}: 72F and sunny",
                    },
                )
            )
            self.assertEqual(len(outputs), 1)
            self.assertEqual(outputs[0]["type"], "function_call_output")
            self.assertIn("72F", outputs[0]["output"])

            final = client.responses.create(
                input=outputs,
                previous_response_id=paused["id"],
                model="mock-gpt",
                tools=[GET_WEATHER_TOOL],
                max_steps=4,
                max_output_tokens=256,
            )
            self.assertEqual(final["status"], "completed")
            text = final.get("output_text") or ""
            self.assertIn("SDK local function ok", text)
            self.assertIn("72F", text)

            retrieved = client.responses.retrieve(paused["id"])
            self.assertEqual(retrieved["status"], "requires_action")
            function_items = [item for item in retrieved.get("output", []) if item.get("type") == "function_call"]
            self.assertEqual(len(function_items), 1)
            self.assertEqual(function_items[0]["name"], "get_weather")
        finally:
            client.close()


if __name__ == "__main__":
    unittest.main()
