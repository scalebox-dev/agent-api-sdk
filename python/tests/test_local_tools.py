from __future__ import annotations

import pytest

from agent_api.local import LocalWorkdir, create_local_workdir_tool_registry, local_workdir_tool_definition


def test_local_workdir_tool_definition_is_one_model_facing_primitive() -> None:
    tool = local_workdir_tool_definition()

    assert tool["type"] == "function"
    assert tool["name"] == "local_workdir"
    assert tool["strict"] is False
    assert tool["parameters"]["required"] == ["action"]
    assert "apply_edits" in tool["parameters"]["properties"]["action"]["enum"]


def test_local_workdir_registry_executes_read_actions(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path)
    workdir.write_text("README.md", "# Demo\nneedle\n")
    registry = create_local_workdir_tool_registry(workdir)

    assert [tool["name"] for tool in registry.definitions()] == ["local_workdir"]
    assert "local_workdir" in registry.handlers()

    listed = registry.execute("local_workdir", {"action": "list", "options": {"recursive": True}})
    assert listed["ok"] is True
    assert any(entry["path"] == "README.md" for entry in listed["entries"])

    grep = registry.handlers()["local_workdir"]({"action": "grep", "pattern": "needle"})
    assert grep["ok"] is True
    assert grep["matches"][0]["path"] == "README.md"

    with pytest.raises(ValueError, match="unknown local workdir tool"):
        registry.execute("other_tool", {"action": "list"})


def test_local_workdir_tool_approval_mode_returns_preview_without_mutation(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path)
    workdir.write_text("notes.txt", "one\ntwo\n")
    registry = create_local_workdir_tool_registry(workdir, access_mode="approval")
    args = {
        "action": "apply_edits",
        "edits": [{"path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"}],
    }

    assert registry.requires_approval("local_workdir", args) is True
    result = registry.execute("local_workdir", args)

    assert result["ok"] is False
    assert result["requires_approval"] is True
    assert result["preview"]["previews"][0]["before"] == ["two"]
    assert workdir.read_text("notes.txt") == "one\ntwo\n"


def test_local_workdir_tool_full_mode_applies_mutations(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path)
    workdir.write_text("notes.txt", "one\ntwo\n")
    registry = create_local_workdir_tool_registry(workdir, access_mode="full")

    result = registry.execute(
        "local_workdir",
        {"action": "apply_edits", "path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"},
    )

    assert registry.requires_approval("local_workdir", {"action": "write", "path": "x.txt", "content": "x"}) is False
    assert result["ok"] is True
    assert result["applied"][0]["path"] == "notes.txt"
    assert workdir.read_text("notes.txt") == "one\nTWO\n"

