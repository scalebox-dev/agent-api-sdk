from __future__ import annotations

import pytest

from agent_api.local import LocalWorkspace, create_local_workspace_tool_registry, local_workspace_tool_definition


def test_local_workspace_tool_definition_is_one_model_facing_primitive() -> None:
    tool = local_workspace_tool_definition()

    assert tool["type"] == "function"
    assert tool["name"] == "local_workspace"
    assert tool["strict"] is False
    assert tool["parameters"]["required"] == ["action"]
    assert "apply_edits" in tool["parameters"]["properties"]["action"]["enum"]


def test_local_workspace_registry_executes_read_actions(tmp_path) -> None:
    workspace = LocalWorkspace(tmp_path)
    workspace.write_text("README.md", "# Demo\nneedle\n")
    registry = create_local_workspace_tool_registry(workspace)

    assert [tool["name"] for tool in registry.definitions()] == ["local_workspace"]
    assert "local_workspace" in registry.handlers()

    listed = registry.execute("local_workspace", {"action": "list", "options": {"recursive": True}})
    assert listed["ok"] is True
    assert any(entry["path"] == "README.md" for entry in listed["entries"])

    grep = registry.handlers()["local_workspace"]({"action": "grep", "pattern": "needle"})
    assert grep["ok"] is True
    assert grep["matches"][0]["path"] == "README.md"

    with pytest.raises(ValueError, match="unknown local workspace tool"):
        registry.execute("other_tool", {"action": "list"})


def test_local_workspace_tool_approval_mode_returns_preview_without_mutation(tmp_path) -> None:
    workspace = LocalWorkspace(tmp_path)
    workspace.write_text("notes.txt", "one\ntwo\n")
    registry = create_local_workspace_tool_registry(workspace, access_mode="approval")
    args = {
        "action": "apply_edits",
        "edits": [{"path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"}],
    }

    assert registry.requires_approval("local_workspace", args) is True
    result = registry.execute("local_workspace", args)

    assert result["ok"] is False
    assert result["requires_approval"] is True
    assert result["preview"]["previews"][0]["before"] == ["two"]
    assert workspace.read_text("notes.txt") == "one\ntwo\n"


def test_local_workspace_tool_full_mode_applies_mutations(tmp_path) -> None:
    workspace = LocalWorkspace(tmp_path)
    workspace.write_text("notes.txt", "one\ntwo\n")
    registry = create_local_workspace_tool_registry(workspace, access_mode="full")

    result = registry.execute(
        "local_workspace",
        {"action": "apply_edits", "path": "notes.txt", "start_line": 2, "end_line": 2, "replacement": "TWO"},
    )

    assert registry.requires_approval("local_workspace", {"action": "write", "path": "x.txt", "content": "x"}) is False
    assert result["ok"] is True
    assert result["applied"][0]["path"] == "notes.txt"
    assert workspace.read_text("notes.txt") == "one\nTWO\n"

