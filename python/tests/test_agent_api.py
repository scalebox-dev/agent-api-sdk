from __future__ import annotations

import asyncio
import json
import tempfile
import unittest
from pathlib import Path

import httpx

from agent_api import AgentAPI, AsyncAgentAPI, RateLimitError, local_skill_from_directory
from agent_api.local_functions import run_local_function_handlers


def response(body: object, status_code: int = 200, headers: dict[str, str] | None = None) -> httpx.Response:
    return httpx.Response(
        status_code,
        json=body,
        headers=headers or {"content-type": "application/json"},
    )


class AgentAPITest(unittest.TestCase):
    def test_responses_create_sends_auth_and_adds_output_text(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["url"] = str(request.url)
            seen["authorization"] = request.headers.get("authorization")
            seen["body"] = json.loads(request.content)
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [
                        {
                            "type": "message",
                            "id": "msg_test",
                            "status": "completed",
                            "role": "assistant",
                            "content": [{"type": "output_text", "text": "hello"}],
                        }
                    ],
                }
            )

        client = AgentAPI(
            api_key="sk-test",
            base_url="https://agent.test",
            http_client=httpx.Client(transport=httpx.MockTransport(handler)),
        )

        result = client.responses.create(input="hello", preset="fast-search")

        self.assertEqual(seen["url"], "https://agent.test/v1/responses")
        self.assertEqual(seen["authorization"], "Bearer sk-test")
        self.assertEqual(seen["body"], {"input": "hello", "stream": False, "preset": "fast-search"})
        self.assertEqual(result["output_text"], "hello")

    def test_responses_create_capability_preferences_and_tools(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["body"] = json.loads(request.content)
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(
            api_key="sk-test",
            base_url="https://agent.test",
            http_client=httpx.Client(transport=httpx.MockTransport(handler)),
        )

        client.responses.create(
            input="hello",
            plan_mode_preference="preferred",
            sub_agent_preference="off",
            tools=[{"name": "smart_web_search"}],
        )

        self.assertEqual(
            seen["body"],
            {
                "input": "hello",
                "stream": False,
                "plan_mode_preference": "preferred",
                "sub_agent_preference": "off",
                "tools": [{"name": "smart_web_search"}],
            },
        )

    def test_agent_create_uses_agent_endpoint(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["url"] = str(request.url)
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        client.agent.create(input="hello")

        self.assertEqual(seen["url"], "https://agent.test/v1/agent")

    def test_auth_device_flow_starts_polls_and_waits(self) -> None:
        calls: list[dict[str, object]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            body = json.loads(request.content) if request.content else None
            calls.append({"path": request.url.path, "body": body})
            if request.url.path == "/v1/auth/device/start":
                return response(
                    {
                        "device_code": "dev_secret",
                        "user_code": "ABCD1234",
                        "verification_uri": "https://www.example.test/auth/device",
                        "verification_uri_complete": "https://www.example.test/auth/device?user_code=ABCD1234",
                        "expires_at": 4102444800,
                        "interval_seconds": 1,
                    }
                )
            if request.url.path == "/v1/auth/device/poll" and len([c for c in calls if c["path"] == "/v1/auth/device/poll"]) == 1:
                return response({"status": "pending", "message": "authorization pending", "interval_seconds": 1, "expires_at": 4102444800})
            return response(
                {
                    "status": "approved",
                    "access_token": "jwt",
                    "refresh_token": "refresh",
                    "access_token_expires_at": 4102441200,
                    "user_id": "user_1",
                    "workspace_id": "wrk_1",
                    "workspace_role": "owner",
                    "scopes": ["responses:create"],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        started = client.auth.start_device_auth(client_name="Agent CLI")
        self.assertEqual(started["device_code"], "dev_secret")
        self.assertEqual(calls[0]["body"], {"client_name": "Agent CLI"})

        approved = client.wait_for_device_auth(device_code=str(started["device_code"]), interval_seconds=1, timeout=3)
        self.assertEqual(approved["status"], "approved")
        self.assertEqual(approved["access_token"], "jwt")
        self.assertEqual(calls[-1]["body"], {"device_code": "dev_secret"})

    def test_responses_and_agent_create_serialize_volume_id(self) -> None:
        bodies: list[dict[str, object]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            bodies.append(json.loads(request.content))
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        client.responses.create(input="hello", volume_id="vol_test")
        client.agent.create(input="hello", volume_id="vol_agent")

        self.assertEqual(bodies[0]["volume_id"], "vol_test")
        self.assertEqual(bodies[1]["volume_id"], "vol_agent")

    def test_responses_and_agent_create_serialize_preferred_sites(self) -> None:
        bodies: list[dict[str, object]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            bodies.append(json.loads(request.content))
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        client.responses.create(input="hello", preferred_sites=["arxiv.org", "nature.com"])
        client.agent.create(input="hello", preferred_sites=["docs.python.org"])

        self.assertEqual(bodies[0]["preferred_sites"], ["arxiv.org", "nature.com"])
        self.assertEqual(bodies[1]["preferred_sites"], ["docs.python.org"])

    def test_responses_and_agent_create_serialize_skills(self) -> None:
        bodies: list[dict[str, object]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            bodies.append(json.loads(request.content))
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        client.responses.create(
            input="hello",
            skills=[{"skill_id": "skl_docs", "branch": "dev"}],
            local_skills=[
                {"local_skill_id": "local_docs", "name": "Docs", "root_hint": "/tmp/docs", "digest": "sha256:abc"}
            ],
        )
        client.agent.create(input="hello", skills=[{"skill_id": "skl_agent"}])

        self.assertEqual(bodies[0]["skills"], [{"skill_id": "skl_docs", "branch": "dev"}])
        self.assertEqual(
            bodies[0]["local_skills"],
            [{"local_skill_id": "local_docs", "name": "Docs", "root_hint": "/tmp/docs", "digest": "sha256:abc"}],
        )
        self.assertEqual(bodies[1]["skills"], [{"skill_id": "skl_agent"}])

    def test_responses_list_with_query(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["url"] = str(request.url)
            return response({"object": "list", "data": [], "has_more": False})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.list(limit=5, page_token="tok")
        self.assertEqual(seen["url"], "https://agent.test/v1/responses?limit=5&page_token=tok")
        self.assertEqual(out["object"], "list")

    def test_responses_retrieve_includes_lineage_and_tool_results(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/responses/resp_abc")
            return response(
                {
                    "id": "resp_abc",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                    "tool_results": [{"tool_name": "web_search", "status": "completed"}],
                    "parent_response_id": "resp_parent",
                    "root_response_id": "resp_root",
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.retrieve("resp_abc")
        self.assertEqual(out["tool_results"][0]["tool_name"], "web_search")
        self.assertEqual(out["parent_response_id"], "resp_parent")

    def test_responses_cancel(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["url"] = str(request.url)
            seen["method"] = request.method
            return response({"interrupted": True})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.cancel("resp_abc")
        self.assertEqual(seen["url"], "https://agent.test/v1/responses/resp_abc/cancel")
        self.assertEqual(seen["method"], "POST")
        self.assertTrue(out["interrupted"])

    def test_responses_list_children(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/responses/resp_parent/children")
            return response({"object": "list", "data": [{"id": "resp_child", "status": "completed", "created_at": 1}]})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.list_children("resp_parent")
        self.assertEqual(out["data"][0]["id"], "resp_child")

    def test_responses_list_events(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/responses/resp_abc/events?after_sequence=3&view=full")
            return response({"data": [{"type": "response.created", "sequence_number": 0}]})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.list_events("resp_abc", after_sequence=3, view="full")
        self.assertEqual(out["data"][0]["type"], "response.created")

    def test_responses_retrieve_volume(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/responses/resp_abc/volume")
            return response({"volume_id": "vol_workspace", "name": "workspace"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.responses.retrieve_volume("resp_abc")
        self.assertEqual(out["volume_id"], "vol_workspace")

    def test_models_list(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/models")
            return response(
                {
                    "object": "list",
                    "data": [
                        {
                            "id": "test/model",
                            "object": "model",
                            "owned_by": "test",
                            "capabilities": {"supports_streaming": True, "supports_tools": True},
                        }
                    ],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.models.list()
        self.assertEqual(out["data"][0]["capabilities"]["supports_streaming"], True)

    def test_presets_list(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/presets")
            return response({"object": "list", "data": [{"preset": "fast-search"}]})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.presets.list()
        self.assertEqual(out["data"][0]["preset"], "fast-search")

    def test_tools_list(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/tools")
            return response(
                {
                    "object": "list",
                    "data": [{"object": "tool", "name": "web_search"}, {"object": "tool", "name": "smart_web_search"}],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        out = client.tools.list()
        self.assertEqual(out["data"][1]["name"], "smart_web_search")

    def test_volumes_resource_routes_and_raw_file_write(self) -> None:
        calls: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            calls.append(request)
            url = str(request.url)
            if url.endswith("/v1/volumes") and request.method == "POST":
                return response({"volume_id": "vol_new", "name": "docs"}, status_code=201)
            if url.endswith("/v1/volumes/vol_delete"):
                return httpx.Response(204)
            if "/entries" in url:
                return response({"object": "list", "entries": [{"path": "dir/file.txt", "is_dir": False, "size": 5}]})
            if "/search" in url:
                return response({"object": "list", "entries": [{"path": "dir/match.txt", "is_dir": False, "size": 7}]})
            if "/files/dir/file%20name.txt" in url and request.method == "GET":
                return response(
                    {
                        "path": "dir/file name.txt",
                        "encoding": "text",
                        "mime_type": "text/plain",
                        "size": 5,
                        "truncated": False,
                        "content": "hello",
                    }
                )
            if "/files/dir/write.txt" in url and request.method == "PUT":
                return response({"path": "dir/write.txt", "size": 11})
            if "/paths/old.txt" in url and request.method == "DELETE":
                return response({"path": "old.txt", "recursive": False})
            if "/vol_123" in url:
                return response({"volume_id": "vol_123", "name": "docs"})
            return response({"object": "list", "data": [{"volume_id": "vol_123", "name": "docs"}], "next_page_token": "next"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        listed = client.volumes.list(limit=1, page_token="tok")
        created = client.volumes.create(name="docs")
        retrieved = client.volumes.retrieve("vol_123")
        entries = client.volumes.list_entries("vol_123", path="dir", limit=2, page_token="etok")
        search = client.volumes.search_entries("vol_123", query="match", path="dir")
        read = client.volumes.read_file("vol_123", "dir/file name.txt", max_bytes=100)
        wrote = client.volumes.write_file("vol_123", "dir/write.txt", "hello world")
        deleted = client.volumes.delete_path("vol_123", "old.txt")
        client.volumes.delete("vol_delete")

        self.assertEqual(str(calls[0].url), "https://agent.test/v1/volumes?limit=1&page_token=tok")
        self.assertEqual(str(calls[1].url), "https://agent.test/v1/volumes")
        self.assertEqual(json.loads(calls[1].content)["name"], "docs")
        self.assertEqual(str(calls[3].url), "https://agent.test/v1/volumes/vol_123/entries?path=dir&limit=2&page_token=etok")
        self.assertEqual(str(calls[4].url), "https://agent.test/v1/volumes/vol_123/search?query=match&path=dir")
        self.assertEqual(str(calls[5].url), "https://agent.test/v1/volumes/vol_123/files/dir/file%20name.txt?max_bytes=100")
        self.assertEqual(calls[6].content, b"hello world")
        self.assertIsNone(calls[6].headers.get("content-type"))
        self.assertEqual(calls[7].method, "DELETE")
        self.assertEqual(calls[8].method, "DELETE")
        self.assertEqual(listed["next_page_token"], "next")
        self.assertEqual(created["volume_id"], "vol_new")
        self.assertEqual(retrieved["volume_id"], "vol_123")
        self.assertEqual(entries["entries"][0]["path"], "dir/file.txt")
        self.assertEqual(search["entries"][0]["path"], "dir/match.txt")
        self.assertEqual(read["encoding"], "text")
        self.assertEqual(read["content"], "hello")
        self.assertEqual(wrote["size"], 11)
        self.assertFalse(deleted["recursive"])

    def test_volumes_update_reconcile_and_directory_routes(self) -> None:
        calls: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            calls.append(request)
            url = str(request.url)
            if "/usage/reconcile" in url:
                return response({"volume_id": "vol_123", "name": "renamed", "bytes_used": 99})
            if url.endswith("/directories") and request.method == "POST":
                return response({"path": "notes/archive"}, status_code=201)
            if request.method == "PATCH":
                return response({"volume_id": "vol_123", "name": "renamed"})
            return response({"volume_id": "vol_123", "name": "docs"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        updated = client.volumes.update("vol_123", name="renamed")
        reconciled = client.volumes.reconcile_usage("vol_123")
        created_dir = client.volumes.create_directory("vol_123", "notes/archive")

        self.assertEqual(calls[0].method, "PATCH")
        self.assertEqual(json.loads(calls[0].content)["name"], "renamed")
        self.assertIn("/usage/reconcile", str(calls[1].url))
        self.assertEqual(calls[2].method, "POST")
        self.assertEqual(updated["name"], "renamed")
        self.assertEqual(reconciled["bytes_used"], 99)
        self.assertEqual(created_dir["path"], "notes/archive")

    def test_volumes_read_file_raw_format(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertIn("format=raw", str(request.url))
            return httpx.Response(
                200,
                content=b"\x01\x02\x03",
                headers={
                    "content-type": "application/octet-stream",
                    "X-Volume-Size": "3",
                    "X-Volume-Truncated": "false",
                },
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))
        raw = client.volumes.read_file("vol_123", "bin/data.bin", format="raw")
        self.assertEqual(raw["size"], 3)
        self.assertFalse(raw["truncated"])
        self.assertEqual(raw["content"], b"\x01\x02\x03")

    def test_volumes_workbench_routes(self) -> None:
        calls: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            calls.append(request)
            url = str(request.url)
            if url.endswith("/summarize"):
                return response(
                    {
                        "summary_path": ".agent-volume/summary.json",
                        "file_count": 2,
                        "total_bytes": 20,
                        "top_paths_by_size": ["a.txt"],
                        "text_previews": [{"path": "a.txt", "size": 10, "preview": "hi"}],
                        "generated_at_unix": 1710000000,
                    }
                )
            if "/grep?" in url:
                return response(
                    {
                        "object": "list",
                        "matches": [{"path": "a.txt", "line_number": 1, "line": "match"}],
                        "files_scanned": 1,
                        "scan_truncated": False,
                    }
                )
            if "/file_lines/readme.md" in url and request.method == "GET":
                return response(
                    {
                        "path": "readme.md",
                        "start_line": 1,
                        "end_line": 2,
                        "total_lines": 10,
                        "lines": ["# Title", ""],
                        "file_truncated": False,
                        "size": 12,
                    }
                )
            if "/file_lines/readme.md" in url and request.method == "PATCH":
                return response(
                    {
                        "path": "readme.md",
                        "start_line": 1,
                        "end_line": 1,
                        "total_lines": 10,
                        "size": 13,
                    }
                )
            return response({"volume_id": "vol_123"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        summary = client.volumes.summarize("vol_123", path="docs")
        grep = client.volumes.grep("vol_123", pattern="match", path="docs", limit=5)
        lines = client.volumes.read_lines("vol_123", "readme.md", start_line=1, end_line=2)
        patched = client.volumes.patch_lines("vol_123", "readme.md", start_line=1, end_line=1, replacement="# Updated")

        self.assertEqual(str(calls[0].url), "https://agent.test/v1/volumes/vol_123/summarize")
        self.assertEqual(json.loads(calls[0].content)["path"], "docs")
        self.assertIn("/grep?pattern=match", str(calls[1].url))
        self.assertIn("start_line=1", str(calls[2].url))
        self.assertEqual(json.loads(calls[3].content)["replacement"], "# Updated")
        self.assertEqual(summary["file_count"], 2)
        self.assertEqual(grep["matches"][0]["line"], "match")
        self.assertEqual(lines["lines"][0], "# Title")
        self.assertEqual(patched["size"], 13)

    def test_volumes_download_archive(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/volumes/vol_123/archive?path=notes%2Fdrafts")
            return httpx.Response(200, content=b"PK\x03\x04", headers={"content-type": "application/zip"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        archive = client.volumes.download_archive("vol_123", path="/notes/drafts/")
        self.assertEqual(archive["path"], "notes/drafts")
        self.assertEqual(archive["content"], b"PK\x03\x04")
        self.assertEqual(archive["content_type"], "application/zip")

    def test_skills_resource_routes(self) -> None:
        calls: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            calls.append(request)
            url = str(request.url)
            if url.endswith("/v1/skills") and request.method == "POST":
                return response({"skill_id": "skl_new", "name": "Docs"}, status_code=201)
            if url.endswith("/v1/skills/discover"):
                return response({"object": "list", "data": [{"skill_id": "skl_1", "branch": "dev", "name": "Docs"}]})
            if url.endswith("/v1/skills/focus"):
                return response({"object": "skill_focus_result", "data": [{"ok": True, "skill_id": "skl_1", "branch": "dev", "skill": {"object": "focused_skill", "skill_id": "skl_1", "branch": "dev", "manifest": "# Skill"}}]})
            if url.endswith("/v1/skills/create_dev"):
                return response({"object": "skill_create_result", "skill": {"skill_id": "skl_created"}, "branch": "dev", "files": [{"path": "SKILL.md", "size": 7}]}, status_code=201)
            if url.endswith("/v1/skills/update_file"):
                return response({"object": "skill_update_result", "data": [{"ok": True, "skill_id": "skl_1", "path": "SKILL.md", "size": 9, "skill": {"skill_id": "skl_1", "branch": "dev"}}]})
            if "/files/guide%20book/SKILL.md" in url and request.method == "GET":
                return response({"path": "guide book/SKILL.md", "branch": "dev", "content": "# Skill", "size": 7})
            if "/files/guide%20book/SKILL.md" in url and request.method == "PUT":
                return response({"path": "guide book/SKILL.md", "branch": "dev", "size": 9})
            if "/files?" in url:
                return response({"object": "list", "entries": [{"path": "SKILL.md", "is_dir": False, "size": 7}]})
            if "/accept_dev" in url:
                return response({"skill_id": "skl_1", "has_dev": False, "main_digest": "sha256:dev"})
            if "/discard_dev" in url:
                return response({"skill_id": "skl_1", "has_dev": False})
            if "/skl_1" in url and request.method == "PATCH":
                return response({"skill_id": "skl_1", "name": "Renamed"})
            if "/skl_1/archive" in url:
                return response({"skill_id": "skl_1", "archived": True})
            if "/skl_1/export" in url:
                return httpx.Response(200, content=b"PK\x03\x04", headers={"content-type": "application/zip"})
            if "/skl_1/import" in url:
                return response({"object": "skill_import_result", "branch": "dev", "file_count": 2, "byte_count": 12, "skill": {"skill_id": "skl_1"}})
            if "/skl_1/diff" in url:
                return response({"object": "skill_branch_diff", "skill_id": "skl_1", "base_branch": "main", "compare_branch": "dev", "summary": {"added": 1, "modified": 0, "deleted": 0, "unchanged": 0}, "files": [{"path": "SKILL.md", "status": "added"}]})
            if "/skl_1" in url and request.method == "DELETE":
                return response({"deleted": True})
            if "/skl_1" in url:
                return response({"skill_id": "skl_1", "name": "Docs"})
            return response({"object": "list", "data": [{"skill_id": "skl_1"}], "next_page_token": "next"})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        listed = client.skills.list(limit=1, page_token="tok")
        created = client.skills.create(name="Docs")
        discovered = client.skills.discover(query="docs", branch="dev")
        focused = client.skills.focus(skills=[{"skill_id": "skl_1", "branch": "dev"}])
        created_dev = client.skills.create_dev(name="New Skill", files=[{"path": "SKILL.md", "content": "# Skill"}])
        updated_file = client.skills.update_file(updates=[{"skill_id": "skl_1", "path": "SKILL.md", "content": "# Skill"}])
        retrieved = client.skills.retrieve("skl_1")
        updated = client.skills.update("skl_1", name="Renamed")
        accepted = client.skills.accept_dev("skl_1", strategy="mirror")
        discarded = client.skills.discard_dev("skl_1")
        files = client.skills.list_files("skl_1", path="guide book", branch="dev")
        read = client.skills.read_file("skl_1", "guide book/SKILL.md", branch="dev", max_bytes=100)
        wrote = client.skills.write_file("skl_1", "guide book/SKILL.md", "# Skill", branch="dev")
        client.skills.delete_file("skl_1", "guide book/SKILL.md", branch="dev")
        archived = client.skills.archive("skl_1")
        exported = client.skills.export_archive("skl_1", path="/examples/", branch="main")
        imported = client.skills.import_archive("skl_1", b"PK", path="examples", branch="dev", replace=True)
        diff = client.skills.diff("skl_1", path="/", max_file_chars=100)
        deleted = client.skills.delete("skl_1")

        self.assertEqual(str(calls[0].url), "https://agent.test/v1/skills?limit=1&page_token=tok")
        self.assertEqual(json.loads(calls[1].content)["name"], "Docs")
        self.assertEqual(str(calls[8].url), "https://agent.test/v1/skills/skl_1/accept_dev?strategy=mirror")
        self.assertEqual(
            str(calls[11].url),
            "https://agent.test/v1/skills/skl_1/files/guide%20book/SKILL.md?branch=dev&max_bytes=100",
        )
        self.assertEqual(calls[12].content, b"# Skill")
        self.assertEqual(listed["next_page_token"], "next")
        self.assertEqual(created["skill_id"], "skl_new")
        self.assertEqual(discovered["data"][0]["branch"], "dev")
        self.assertEqual(focused["data"][0]["skill"]["manifest"], "# Skill")
        self.assertEqual(created_dev["skill"]["skill_id"], "skl_created")
        self.assertEqual(updated_file["data"][0]["size"], 9)
        self.assertEqual(retrieved["skill_id"], "skl_1")
        self.assertEqual(updated["name"], "Renamed")
        self.assertEqual(accepted["main_digest"], "sha256:dev")
        self.assertEqual(discarded["has_dev"], False)
        self.assertEqual(files["entries"][0]["path"], "SKILL.md")
        self.assertEqual(read["content"], "# Skill")
        self.assertEqual(wrote["size"], 9)
        self.assertEqual(archived["archived"], True)
        self.assertEqual(exported["content"], b"PK\x03\x04")
        self.assertEqual(imported["file_count"], 2)
        self.assertEqual(diff["summary"]["added"], 1)
        self.assertEqual(deleted["deleted"], True)
        self.assertEqual(str(calls[14].url), "https://agent.test/v1/skills/skl_1/archive")
        self.assertEqual(str(calls[15].url), "https://agent.test/v1/skills/skl_1/export?path=examples&branch=main")
        self.assertEqual(str(calls[16].url), "https://agent.test/v1/skills/skl_1/import?path=examples&branch=dev&replace=True")
        self.assertEqual(str(calls[17].url), "https://agent.test/v1/skills/skl_1/diff?max_file_chars=100")
        self.assertEqual(str(calls[18].url), "https://agent.test/v1/skills/skl_1")
        self.assertEqual(calls[18].method, "DELETE")

    def test_skills_directory_sync_helpers(self) -> None:
        archive: bytes = b""

        def handler(request: httpx.Request) -> httpx.Response:
            nonlocal archive
            url = str(request.url)
            if "/import" in url:
                archive = request.content
                return response({"object": "skill_import_result", "branch": "dev", "file_count": 2, "byte_count": len(archive), "skill": {"skill_id": "skl_1"}})
            if "/export" in url:
                return httpx.Response(200, content=archive, headers={"content-type": "application/zip"})
            raise AssertionError(f"unexpected URL {url}")

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))
        with tempfile.TemporaryDirectory(prefix="agent-skill-sync-") as tmp:
            root = Path(tmp)
            source = root / "source"
            target = root / "target"
            (source / "examples").mkdir(parents=True)
            (source / "SKILL.md").write_text("# Local Skill\n", encoding="utf-8")
            (source / "examples" / "demo.md").write_text("demo\n", encoding="utf-8")

            pushed = client.skills.push_directory("skl_1", source, branch="dev", replace=True)
            pulled = client.skills.pull_directory("skl_1", target, branch="dev", replace=True)

            self.assertEqual(pushed["file_count"], 2)
            self.assertEqual(pulled["file_count"], 2)
            self.assertEqual((target / "SKILL.md").read_text(encoding="utf-8"), "# Local Skill\n")
            self.assertEqual((target / "examples" / "demo.md").read_text(encoding="utf-8"), "demo\n")

    def test_local_skill_from_directory(self) -> None:
        with tempfile.TemporaryDirectory(prefix="agent-skill-") as tmp:
            root = Path(tmp)
            (root / "nested").mkdir()
            (root / "SKILL.md").write_text("# Skill\n", encoding="utf-8")
            (root / "nested" / "notes.txt").write_text("hello\n", encoding="utf-8")

            descriptor = local_skill_from_directory(root, id="local_test", name="Local Test")

        self.assertEqual(descriptor["local_skill_id"], "local_test")
        self.assertEqual(descriptor["name"], "Local Test")
        self.assertTrue(str(descriptor["root_hint"]).endswith(tmp))
        self.assertRegex(descriptor["digest"], r"^sha256:[a-f0-9]{64}$")

    def test_streaming_responses_parse_sse(self) -> None:
        def handler(_request: httpx.Request) -> httpx.Response:
            return httpx.Response(
                200,
                headers={"content-type": "text/event-stream"},
                text=(
                    'event: response.output_text.delta\n'
                    'data: {"type":"response.output_text.delta","sequence_number":1,"delta":"hi"}\n\n'
                    'data: {"type":"response.requires_action","sequence_number":2,'
                    '"response":{"status":"requires_action"}}\n\n'
                    'data: {"type":"response.tool.invocation.completed","sequence_number":3,'
                    '"tool_result":{"tool_name":"web_search"}}\n\n'
                ),
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        events = list(client.responses.create(input="hello", stream=True))

        self.assertEqual(len(events), 3)
        self.assertEqual(events[0]["type"], "response.output_text.delta")
        self.assertEqual(events[0]["delta"], "hi")
        self.assertEqual(events[1]["type"], "response.requires_action")
        self.assertEqual(events[1]["response"]["status"], "requires_action")
        self.assertEqual(events[2]["type"], "response.tool.invocation.completed")
        self.assertEqual(events[2]["tool_result"]["tool_name"], "web_search")

    def test_status_error_exposes_status_and_body(self) -> None:
        def handler(_request: httpx.Request) -> httpx.Response:
            return response({"error": {"message": "rate limited", "code": "rate_limit_exceeded"}}, status_code=429)

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))

        with self.assertRaises(RateLimitError) as raised:
            client.responses.create(input="hello")

        self.assertEqual(raised.exception.status_code, 429)
        self.assertEqual(raised.exception.body["error"]["message"], "rate limited")

    def test_user_agent_header_is_set(self) -> None:
        seen: dict[str, object] = {}

        def handler(request: httpx.Request) -> httpx.Response:
            seen["user_agent"] = request.headers.get("user-agent")
            return response({"object": "list", "data": []})

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))
        client.models.list()
        self.assertTrue(str(seen["user_agent"]).startswith("cloudsway-agent/"))

    def test_list_iterator_paginates(self) -> None:
        calls = {"count": 0}

        def handler(request: httpx.Request) -> httpx.Response:
            calls["count"] += 1
            if calls["count"] == 1:
                self.assertEqual(str(request.url), "https://agent.test/v1/responses?limit=1")
                return response(
                    {
                        "object": "list",
                        "data": [{"id": "resp_1", "status": "completed", "created_at": 1}],
                        "has_more": True,
                        "next_page_token": "tok",
                    }
                )
            self.assertEqual(str(request.url), "https://agent.test/v1/responses?limit=1&page_token=tok")
            return response(
                {
                    "object": "list",
                    "data": [{"id": "resp_2", "status": "completed", "created_at": 2}],
                    "has_more": False,
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))
        ids = [item["id"] for item in client.responses.list_iterator(limit=1)]
        self.assertEqual(ids, ["resp_1", "resp_2"])

    def test_retries_retryable_503(self) -> None:
        attempts = {"count": 0}

        def handler(_request: httpx.Request) -> httpx.Response:
            attempts["count"] += 1
            if attempts["count"] == 1:
                return response({"error": {"message": "upstream unavailable"}}, status_code=503)
            return response(
                {
                    "id": "resp_test",
                    "object": "response",
                    "created_at": 1,
                    "status": "completed",
                    "model": "test/model",
                    "output": [],
                }
            )

        client = AgentAPI(base_url="https://agent.test", http_client=httpx.Client(transport=httpx.MockTransport(handler)))
        client.responses.create(input="hello")
        self.assertEqual(attempts["count"], 2)


class LocalFunctionCallingSDKTest(unittest.TestCase):
    def test_pause_and_resume_with_mock_transport(self) -> None:
        calls: list[dict[str, object]] = []

        def handler(request: httpx.Request) -> httpx.Response:
            body = json.loads(request.content)
            calls.append(body)
            if len(calls) == 1:
                return response(
                    {
                        "id": "resp_paused",
                        "object": "response",
                        "created_at": 1,
                        "status": "requires_action",
                        "model": "mock-gpt",
                        "output": [
                            {
                                "type": "function_call",
                                "id": "fc_1",
                                "status": "in_progress",
                                "name": "get_weather",
                                "call_id": "call_1",
                                "arguments": json.dumps({"city": "Boston"}),
                            }
                        ],
                    }
                )
            self.assertEqual(body.get("previous_response_id"), "resp_paused")
            input_items = body.get("input")
            self.assertIsInstance(input_items, list)
            self.assertEqual(input_items[0]["type"], "function_call_output")
            return response(
                {
                    "id": "resp_final",
                    "object": "response",
                    "created_at": 2,
                    "status": "completed",
                    "model": "mock-gpt",
                    "output": [
                        {
                            "type": "message",
                            "id": "msg_1",
                            "status": "completed",
                            "role": "assistant",
                            "content": [{"type": "output_text", "text": "Boston is 72F and sunny."}],
                        }
                    ],
                }
            )

        client = AgentAPI(
            api_key="sk-test",
            base_url="https://agent.test",
            http_client=httpx.Client(transport=httpx.MockTransport(handler)),
        )
        tool = {
            "type": "function",
            "name": "get_weather",
            "parameters": {"type": "object", "properties": {"city": {"type": "string"}}},
        }

        paused = client.responses.create(model="mock-gpt", input="weather?", tools=[tool])
        self.assertEqual(paused["status"], "requires_action")

        outputs = asyncio.run(
            run_local_function_handlers(
                paused,
                {"get_weather": lambda args: f"{args['city']}: 72F and sunny"},
            )
        )
        final = client.responses.create(
            input=outputs,
            previous_response_id=paused["id"],
            model="mock-gpt",
            tools=[tool],
        )
        self.assertEqual(final["status"], "completed")
        self.assertEqual(final["output_text"], "Boston is 72F and sunny.")
        self.assertEqual(len(calls), 2)


class AsyncAgentAPITest(unittest.IsolatedAsyncioTestCase):
    async def test_async_presets_and_tools(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            if str(request.url).endswith("/v1/presets"):
                return response({"object": "list", "data": [{"preset": "pro-search"}]})
            if str(request.url).endswith("/v1/tools"):
                return response({"object": "list", "data": [{"object": "tool", "name": "fetch_url"}]})
            return response({}, status_code=404)

        client = AsyncAgentAPI(base_url="https://agent.test", http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

        presets = await client.presets.list()
        tools = await client.tools.list()
        await client.close()

        self.assertEqual(presets["data"][0]["preset"], "pro-search")
        self.assertEqual(tools["data"][0]["name"], "fetch_url")

    async def test_async_volumes_list(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            self.assertEqual(str(request.url), "https://agent.test/v1/volumes?limit=1")
            return response({"object": "list", "data": [{"volume_id": "vol_async"}]})

        client = AsyncAgentAPI(base_url="https://agent.test", http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

        out = await client.volumes.list(limit=1)
        await client.close()

        self.assertEqual(out["data"][0]["volume_id"], "vol_async")

    async def test_async_responses_list_iterator(self) -> None:
        pages = iter(
            [
                {"object": "list", "data": [{"id": "resp_1", "status": "completed"}], "has_more": True, "next_page_token": "tok2"},
                {"object": "list", "data": [{"id": "resp_2", "status": "completed"}], "has_more": False},
            ]
        )

        def handler(_request: httpx.Request) -> httpx.Response:
            return response(next(pages))

        client = AsyncAgentAPI(base_url="https://agent.test", http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

        ids = [item["id"] async for item in client.responses.list_iterator(limit=1)]
        await client.close()

        self.assertEqual(ids, ["resp_1", "resp_2"])


if __name__ == "__main__":
    unittest.main()
