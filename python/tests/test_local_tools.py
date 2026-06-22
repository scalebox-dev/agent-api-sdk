from __future__ import annotations

import pytest

from agent_api.local import (
    LocalWorkdir,
    create_local_shell_tool_registry,
    create_local_workdir_tool_registry,
    local_shell_tool_definition,
    local_workdir_tool_definition,
)


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


def test_local_shell_registry_exposes_one_model_facing_primitive(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path)
    definitions = registry.definitions()

    assert [tool["name"] for tool in definitions] == ["local_shell"]
    assert definitions[0]["type"] == "function"
    assert definitions[0]["parameters"]["properties"]["command"]["type"] == "string"
    assert "platform=" in definitions[0]["description"]
    assert "access_mode=approval" in definitions[0]["description"]
    assert "not a filesystem sandbox" in definitions[0]["description"]

    result = registry.execute("local_shell", {"command": "printf shell-ready", "description": "Smoke test shell"})
    assert result["ok"] is False
    assert result["requires_approval"] is True
    assert result["command"] == "printf shell-ready"
    assert registry.requires_approval("local_shell", {}) is True


def test_local_shell_full_mode_executes_commands_in_configured_cwd(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path, access_mode="full")

    result = registry.execute(
        "local_shell",
        {"command": "printf shell-output > result.txt && printf done", "description": "Write result file"},
    )

    assert result["ok"] is True
    assert result["exit_code"] == 0
    assert "done" in result["output"]
    assert (tmp_path / "result.txt").read_text() == "shell-output"
    assert registry.requires_approval("local_shell", {}) is False


def test_local_shell_rejects_workdir_traversal(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path, access_mode="full")

    with pytest.raises(ValueError, match="workdir must stay inside"):
        registry.execute("local_shell", {"command": "pwd", "workdir": ".."})


def test_local_shell_tool_definition_can_be_renamed_for_host_integrations() -> None:
    tool = local_shell_tool_definition(
        "host_command",
        access_mode="full",
        cwd="/tmp/example",
        platform="linux",
        shell="bash",
        timeout_ms=1000,
        max_output_bytes=2048,
    )

    assert tool["name"] == "host_command"
    assert "local shell" in tool["description"]
    assert "platform=linux" in tool["description"]
    assert "shell=bash" in tool["description"]
    assert "access_mode=full" in tool["description"]
    assert "default_timeout_ms=1000" in tool["description"]
    assert "max_output_bytes=2048" in tool["description"]
