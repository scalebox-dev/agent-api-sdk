from __future__ import annotations

import json
import asyncio

import pytest

from agent_api.local_skills import local_skill_from_directory, pending_local_skill_calls, run_local_skill_handlers


def test_run_local_skill_handlers_focuses_local_skill(tmp_path) -> None:
    skill_dir = tmp_path / "release-triage"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_text("# Release Triage\n\nMarker: local-skill-marker\n", encoding="utf-8")
    (skill_dir / "examples.md").write_text("example\n", encoding="utf-8")
    descriptor = local_skill_from_directory(skill_dir, id="local_release_triage", name="Release Triage")
    skill_ref = descriptor["skill_ref"]
    response = {
        "id": "resp_1",
        "object": "response",
        "status": "requires_action",
        "model": "m",
        "output": [
            {
                "type": "function_call",
                "id": "fc_1",
                "status": "in_progress",
                "name": "skill_focus",
                "call_id": "call_skill_1",
                "arguments": json.dumps({"skills": [{"skill_ref": skill_ref, "paths": ["examples.md"]}], "max_manifest_chars": 4096, "max_file_chars": 100}),
            }
        ],
    }

    pending = pending_local_skill_calls(response)
    assert len(pending) == 1

    outputs = asyncio.run(run_local_skill_handlers(response, [descriptor]))
    assert len(outputs) == 1
    assert outputs[0]["type"] == "function_call_output"
    assert outputs[0]["call_id"] == "call_skill_1"

    payload = json.loads(outputs[0]["output"])
    item = payload["data"][0]
    assert item["ok"] is True
    assert item["skill_ref"] == skill_ref
    assert item["skill"]["skill_ref"] == skill_ref
    assert "local_skill_id" not in item["skill"]
    assert "local-skill-marker" in item["skill"]["manifest"]
    assert {entry["path"] for entry in item["skill"]["entries"]} == {"SKILL.md", "examples.md"}
    assert item["skill"]["files"][0]["path"] == "examples.md"
    assert item["skill"]["files"][0]["content"] == "example\n"


def test_local_skill_from_directory_includes_initial_manifest(tmp_path) -> None:
    skill_dir = tmp_path / "release-triage"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_text("# Release Triage\n\nMarker: initial-focus\n", encoding="utf-8")

    descriptor = local_skill_from_directory(skill_dir, id="local_release_triage", max_manifest_chars=24)

    assert descriptor["local_skill_id"] == "local_release_triage"
    assert descriptor["skill_ref"].startswith("local::local_release_triage@")
    assert descriptor["skill_ref"].endswith("::main")
    assert "initial" not in descriptor["manifest"]
    assert descriptor["manifest"].startswith("# Release Triage")
    assert descriptor["manifest_truncated"] is True


def test_run_local_skill_handlers_returns_file_level_errors(tmp_path) -> None:
    skill_dir = tmp_path / "release-triage"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_text("# Release Triage\n", encoding="utf-8")
    descriptor = local_skill_from_directory(skill_dir, id="local_release_triage")
    skill_ref = descriptor["skill_ref"]
    response = {
        "id": "resp_1",
        "object": "response",
        "status": "requires_action",
        "model": "m",
        "output": [
            {
                "type": "function_call",
                "id": "fc_1",
                "status": "in_progress",
                "name": "skill_focus",
                "call_id": "call_skill_1",
                "arguments": json.dumps({"skills": [{"skill_ref": skill_ref, "paths": ["missing.md", "../outside.md"]}]}),
            }
        ],
    }

    outputs = asyncio.run(run_local_skill_handlers(response, [descriptor]))
    payload = json.loads(outputs[0]["output"])
    files = payload["data"][0]["skill"]["files"]
    assert [file["error"]["code"] for file in files] == ["skill_file_not_found", "invalid_skill_file_path"]


def test_run_local_skill_handlers_can_skip_manifest_on_followup(tmp_path) -> None:
    skill_dir = tmp_path / "release-triage"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_text("# Release Triage\n\nMarker: should-not-return\n", encoding="utf-8")
    (skill_dir / "examples.md").write_text("followup example\n", encoding="utf-8")
    descriptor = local_skill_from_directory(skill_dir, id="local_release_triage")
    skill_ref = descriptor["skill_ref"]
    response = {
        "id": "resp_1",
        "object": "response",
        "status": "requires_action",
        "model": "m",
        "output": [
            {
                "type": "function_call",
                "id": "fc_1",
                "status": "in_progress",
                "name": "skill_focus",
                "call_id": "call_skill_1",
                "arguments": json.dumps({"skills": [{"skill_ref": skill_ref, "include_manifest": False, "paths": ["examples.md"]}]}),
            }
        ],
    }

    outputs = asyncio.run(run_local_skill_handlers(response, [descriptor]))
    payload = json.loads(outputs[0]["output"])
    skill = payload["data"][0]["skill"]
    assert skill["manifest"] == ""
    assert skill["manifest_truncated"] is False
    assert skill["files"][0]["content"] == "followup example\n"


def test_run_local_skill_handlers_truncates_by_characters(tmp_path) -> None:
    skill_dir = tmp_path / "unicode-skill"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_text("ab界🙂cd", encoding="utf-8")
    descriptor = local_skill_from_directory(skill_dir, id="unicode_skill")
    skill_ref = descriptor["skill_ref"]
    response = {
        "id": "resp_1",
        "object": "response",
        "status": "requires_action",
        "model": "m",
        "output": [
            {
                "type": "function_call",
                "id": "fc_1",
                "status": "in_progress",
                "name": "skill_focus",
                "call_id": "call_skill_1",
                "arguments": json.dumps({"skills": [{"skill_ref": skill_ref}], "max_manifest_chars": 4}),
            }
        ],
    }

    outputs = asyncio.run(run_local_skill_handlers(response, [descriptor]))
    payload = json.loads(outputs[0]["output"])
    skill = payload["data"][0]["skill"]
    assert skill["manifest"] == "ab界🙂"
    assert skill["manifest_truncated"] is True


def test_run_local_skill_handlers_rejects_invalid_utf8_manifest(tmp_path) -> None:
    skill_dir = tmp_path / "broken-skill"
    skill_dir.mkdir()
    (skill_dir / "SKILL.md").write_bytes(b"ok\xff")
    with pytest.raises(UnicodeDecodeError):
        local_skill_from_directory(skill_dir, id="broken_skill")
