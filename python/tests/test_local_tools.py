from __future__ import annotations

import json
import os
import stat

import pytest

from agent_api.local import (
    IsolatorLocalShellRunner,
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

    file_grep = registry.execute("local_workdir", {"action": "grep", "path": "README.md", "pattern": "needle"})
    assert file_grep["ok"] is True
    assert len(file_grep["matches"]) == 1
    assert file_grep["matches"][0]["path"] == "README.md"

    missing = registry.execute("local_workdir", {"action": "grep", "path": "missing.txt", "pattern": "needle"})
    assert missing["ok"] is False
    assert missing["action"] == "grep"
    assert "missing.txt" in str(missing["path"]) or "missing" in missing["error"]

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
    assert result["changed_files"] == ["notes.txt"]
    assert result["edit_count"] == 1
    assert "backups" not in result
    assert workdir.read_text("notes.txt") == "one\nTWO\n"


def test_local_shell_registry_exposes_one_model_facing_primitive(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path)
    definitions = registry.definitions()

    assert [tool["name"] for tool in definitions] == ["local_shell"]
    assert definitions[0]["type"] == "function"
    assert definitions[0]["parameters"]["properties"]["command"]["type"] == "string"
    assert "platform=" in definitions[0]["description"]
    assert "access_mode=approval" in definitions[0]["description"]
    assert "isolation_driver=direct" in definitions[0]["description"]
    assert "not a filesystem sandbox" in definitions[0]["description"]

    result = registry.execute("local_shell", {"command": "printf shell-ready", "description": "Smoke test shell"})
    assert result["ok"] is False
    assert result["requires_approval"] is True
    assert result["command"] == "printf shell-ready"
    assert result["shell_isolation"]["driver"] == "direct"
    assert result["shell_isolation"]["isolated"] is False
    assert registry.requires_approval("local_shell", {}) is True


def test_local_shell_full_mode_executes_commands_in_configured_cwd(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path, access_mode="full")

    result = registry.execute(
        "local_shell",
        {"command": "printf shell-output > result.txt && printf done", "description": "Write result file"},
    )

    assert result["ok"] is True
    assert result["exit_code"] == 0
    assert result["shell_isolation"]["driver"] == "direct"
    assert result["shell_isolation"]["isolated"] is False
    assert result["shell_isolation"]["fallback"] is False
    assert "done" in result["output"]
    assert (tmp_path / "result.txt").read_text() == "shell-output"
    assert registry.requires_approval("local_shell", {}) is False


def test_local_shell_none_isolation_is_explicit_direct_host_execution(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path, access_mode="full", isolation="none")

    result = registry.execute("local_shell", {"command": "printf direct"})

    assert result["ok"] is True
    assert result["shell_isolation"]["executor"] == "direct"
    assert result["shell_isolation"]["driver"] == "direct"
    assert result["shell_isolation"]["isolated"] is False
    assert result["shell_isolation"]["fallback"] is False
    assert "Direct host execution" in " ".join(result["shell_isolation"]["warnings"])


def test_local_shell_auto_isolation_falls_back_to_direct_executor(tmp_path) -> None:
    registry = create_local_shell_tool_registry(cwd=tmp_path, access_mode="full", isolation="auto")
    definition = registry.definitions()[0]
    assert "fallback=true" in definition["description"]

    result = registry.execute("local_shell", {"command": "printf isolated-fallback"})

    assert result["ok"] is True
    assert result["shell_isolation"]["executor"] == "direct"
    assert result["shell_isolation"]["driver"] == "direct"
    assert result["shell_isolation"]["isolated"] is False
    assert result["shell_isolation"]["fallback"] is True
    assert "falling back to direct" in " ".join(result["shell_isolation"]["warnings"])


def test_local_shell_isolation_options_report_without_direct_enforcement(tmp_path) -> None:
    registry = create_local_shell_tool_registry(
        cwd=tmp_path,
        access_mode="full",
        isolation="auto",
        isolation_options={
            "filesystem": "workdir-readwrite",
            "network": "blocked",
            "env": "minimal",
            "resources": {"memoryMb": 256, "cpuCount": 1},
        },
    )
    definition = registry.definitions()[0]
    assert "filesystem=workdir-readwrite" in definition["description"]
    assert "network=blocked" in definition["description"]
    assert "memory_mb=256" in definition["description"]

    result = registry.execute("local_shell", {"command": "printf options"})

    assert result["ok"] is True
    assert result["shell_isolation"]["requested"] == {
        "filesystem": "workdir-readwrite",
        "network": "blocked",
        "env": "minimal",
        "resources": {"memoryMb": 256, "cpuCount": 1},
    }
    assert result["shell_isolation"]["guarantees"]["filesystem"] == "none"
    assert result["shell_isolation"]["guarantees"]["network"] == "allowed"
    assert "not enforced by direct execution" in " ".join(result["shell_isolation"]["warnings"])


def test_local_shell_required_isolation_rejects_missing_isolating_runner(monkeypatch) -> None:
    monkeypatch.delenv("AGENT_ISOLATOR_PATH", raising=False)
    with pytest.raises(RuntimeError, match="executable path is not configured|AGENT_ISOLATOR_PATH"):
        create_local_shell_tool_registry(access_mode="full", isolation="required")

    class Runner:
        def run(self, request):  # noqa: ANN001, ANN201
            raise AssertionError("should not run")

    with pytest.raises(ValueError, match="does not report isolation"):
        create_local_shell_tool_registry(access_mode="full", isolation="required", runner=Runner())


def test_local_shell_required_isolation_can_run_through_agent_isolator_protocol(tmp_path) -> None:
    executable = _fake_agent_isolator(tmp_path)
    registry = create_local_shell_tool_registry(
        cwd=tmp_path,
        access_mode="full",
        isolation="required",
        isolation_options={"filesystem": "workdir-readwrite", "network": "allowed", "env": "inherit"},
        isolator={"executable_path": executable, "driver": "fake"},
    )
    definition = registry.definitions()[0]
    assert "isolation_driver=fake-isolator" in definition["description"]

    result = registry.execute("local_shell", {"command": "printf through-isolator"})

    assert result["ok"] is True
    assert result["output"] == "through-isolator"
    assert result["shell_isolation"]["executor"] == "isolator"
    assert result["shell_isolation"]["driver"] == "fake-isolator"
    assert result["shell_isolation"]["isolated"] is True


def test_local_shell_required_isolation_fails_closed_when_agent_isolator_unavailable(tmp_path) -> None:
    with pytest.raises((FileNotFoundError, RuntimeError), match="agent-isolator|No such file|not found"):
        create_local_shell_tool_registry(
            cwd=tmp_path,
            access_mode="full",
            isolation="required",
            isolator={"executable_path": tmp_path / "missing-agent-isolator"},
        )


def test_local_shell_auto_isolation_uses_explicit_env_path(tmp_path, monkeypatch) -> None:
    executable = _fake_agent_isolator(tmp_path)
    monkeypatch.setenv("AGENT_ISOLATOR_PATH", str(executable))
    registry = create_local_shell_tool_registry(
        cwd=tmp_path,
        access_mode="full",
        isolation="auto",
        isolation_options={"filesystem": "workdir-readwrite", "network": "allowed", "env": "inherit"},
        isolator={"driver": "fake"},
    )
    result = registry.execute("local_shell", {"command": "printf through-env-isolator"})
    assert result["shell_isolation"]["executor"] == "isolator"
    assert result["shell_isolation"]["driver"] == "fake-isolator"


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


def _fake_agent_isolator(tmp_path) -> str:
    executable = tmp_path / "fake-agent-isolator"
    executable.write_text(
        """#!/usr/bin/env python3
import json
import sys

request = json.load(sys.stdin)
status = {
    "executor": "isolator",
    "driver": "fake-isolator",
    "isolated": True,
    "fallback": False,
    "requested": {"filesystem": "workdir-readwrite", "network": "allowed", "env": "inherit", "resources": {}},
    "guarantees": {"filesystem": "workdir-mounted", "network": "allowed", "user": "namespace-user", "process": "pid-namespace", "resources": "timeout-only"},
    "warnings": [],
}
if request.get("method") == "status":
    print(json.dumps({"id": request.get("id"), "result": {"version": "fake", "driver": "fake-isolator", "status": status, "drivers": []}}))
elif request.get("method") == "run":
    params = request.get("params", {})
    print(json.dumps({"id": request.get("id"), "result": {
        "ok": True,
        "action": "run",
        "command": params.get("command"),
        "description": params.get("description"),
        "cwd": params.get("cwd"),
        "exit_code": 0,
        "signal": None,
        "stdout": "through-isolator",
        "stderr": "",
        "output": "through-isolator",
        "duration_ms": 1,
        "timed_out": False,
        "truncated": False,
        "shell_isolation": status,
    }}))
else:
    print(json.dumps({"id": request.get("id"), "error": {"code": "unknown_method", "message": "unknown method"}}))
    sys.exit(1)
""",
        encoding="utf-8",
    )
    executable.chmod(executable.stat().st_mode | stat.S_IXUSR)
    return os.fspath(executable)
