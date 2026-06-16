from __future__ import annotations

import os
import unittest

from agent_api import AgentAPI


def integration_enabled() -> bool:
    return os.environ.get("AGENT_API_INTEGRATION") == "1"


def integration_client() -> AgentAPI:
    api_key = os.environ.get("AGENT_API_KEY")
    if not api_key:
        raise unittest.SkipTest("AGENT_API_KEY is required for integration tests")
    base_url = (os.environ.get("AGENT_API_BASE_URL") or "https://api.agentsway.dev").rstrip("/")
    return AgentAPI(api_key=api_key, base_url=base_url, timeout=120.0)


@unittest.skipUnless(integration_enabled(), "set AGENT_API_INTEGRATION=1 to run live API tests")
class AgentAPIIntegrationTest(unittest.TestCase):
    def test_discovery_endpoints(self) -> None:
        client = integration_client()
        try:
            models = client.models.list()
            self.assertEqual(models["object"], "list")
            self.assertIsInstance(models["data"], list)
            if models["data"]:
                caps = models["data"][0].get("capabilities", {})
                self.assertTrue("supports_streaming" in caps or not caps)

            presets = client.presets.list()
            self.assertEqual(presets["object"], "list")
            self.assertTrue(any(row.get("preset") == "fast-search" for row in presets["data"]))

            tools = client.tools.list()
            self.assertEqual(tools["object"], "list")
            tool_names = [row["name"] for row in tools["data"]]
            self.assertIn("web_search", tool_names)
            self.assertIn("smart_web_search", tool_names)
        finally:
            client.close()

    def test_create_retrieve_list_events(self) -> None:
        client = integration_client()
        try:
            created = client.responses.create(
                preset="fast-search",
                input="Reply with exactly: SDK integration ok",
                max_output_tokens=64,
            )
            self.assertTrue(created["id"].startswith("resp_"))
            self.assertEqual(created["object"], "response")
            self.assertIn(created["status"], {"completed", "failed", "in_progress", "cancelled"})
            self.assertIsInstance(created.get("output_text"), str)

            retrieved = client.responses.retrieve(created["id"])
            self.assertEqual(retrieved["id"], created["id"])

            listed = client.responses.list(limit=5)
            self.assertEqual(listed["object"], "list")
            self.assertTrue(any(row["id"] == created["id"] for row in listed["data"]))

            children = client.responses.list_children(created["id"])
            self.assertEqual(children["object"], "list")

            events = client.responses.list_events(created["id"], view="timeline")
            self.assertIsInstance(events["data"], list)
            self.assertGreater(len(events["data"]), 0)
            self.assertTrue(
                any(event["type"] in {"response.created", "response.completed"} for event in events["data"])
            )
        finally:
            client.close()

    def test_streaming_lifecycle(self) -> None:
        client = integration_client()
        try:
            events = list(
                client.responses.create(
                    preset="fast-search",
                    input="Say hi in one short sentence.",
                    max_output_tokens=64,
                    stream=True,
                )
            )
            types = {event["type"] for event in events}
            self.assertTrue("response.created" in types or "response.in_progress" in types)
            self.assertTrue("response.completed" in types or "response.failed" in types)
        finally:
            client.close()

    def test_agent_endpoint(self) -> None:
        client = integration_client()
        try:
            created = client.agent.create(
                preset="fast-search",
                input="Reply with exactly: agent endpoint ok",
                max_output_tokens=64,
            )
            self.assertTrue(created["id"].startswith("resp_"))
            self.assertIsInstance(created.get("output_text"), str)
        finally:
            client.close()


if __name__ == "__main__":
    unittest.main()
