from __future__ import annotations

import os
import platform
import shlex
import subprocess
import time
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Literal, Protocol, cast

from agent_api.types.tools import FunctionTool, Tool

from .workdir import LocalWorkdir

LocalShellAccessMode = Literal["approval", "full"]


@dataclass(frozen=True)
class LocalShellRequest:
    command: str
    description: str | None = None
    workdir: str | None = None
    timeout_ms: int | None = None
    env: Mapping[str, str | None] | None = None


class LocalCommandRunner(Protocol):
    def run(self, request: LocalShellRequest) -> dict[str, Any]: ...


class HostLocalShellRunner:
    def __init__(
        self,
        *,
        cwd: str | Path | None = None,
        shell: str | bool | None = None,
        timeout_ms: int = 120_000,
        max_output_bytes: int = 128 * 1024,
        env: Mapping[str, str | None] | None = None,
    ) -> None:
        self.cwd = Path(cwd or os.getcwd()).resolve()
        self.shell = shell if shell is not None else True
        self.timeout_ms = _positive_int(timeout_ms, "timeout_ms")
        self.max_output_bytes = _positive_int(max_output_bytes, "max_output_bytes")
        self.env = {**os.environ}
        if env:
            for key, value in env.items():
                if value is None:
                    self.env.pop(key, None)
                else:
                    self.env[key] = value

    def run(self, request: LocalShellRequest) -> dict[str, Any]:
        command = _required_string(request.command, "command")
        cwd = _resolve_contained_path(self.cwd, request.workdir)
        timeout_ms = self.timeout_ms if request.timeout_ms is None else _positive_int(request.timeout_ms, "timeout_ms")
        env = {**self.env}
        if request.env:
            for key, value in request.env.items():
                if value is None:
                    env.pop(key, None)
                else:
                    env[key] = value

        started = time.monotonic()
        timed_out = False
        try:
            shell_enabled = self.shell is not False
            completed = subprocess.run(
                command if shell_enabled else shlex.split(command),
                cwd=cwd,
                env=env,
                shell=shell_enabled,
                executable=self.shell if isinstance(self.shell, str) and self.shell.strip() else None,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                timeout=timeout_ms / 1000,
                check=False,
            )
            exit_code: int | None = completed.returncode
            stdout_bytes = completed.stdout or b""
            stderr_bytes = completed.stderr or b""
        except subprocess.TimeoutExpired as exc:
            timed_out = True
            exit_code = None
            stdout_bytes = exc.stdout or b""
            stderr_bytes = exc.stderr or b""

        stdout, stdout_truncated = _bounded_decode(stdout_bytes, self.max_output_bytes)
        stderr, stderr_truncated = _bounded_decode(stderr_bytes, self.max_output_bytes)
        output = "\n".join(part for part in (stdout, stderr) if part) or "(no output)"
        return {
            "ok": True,
            "action": "run",
            "command": command,
            "description": request.description,
            "cwd": str(cwd),
            "exit_code": exit_code,
            "signal": None,
            "stdout": stdout,
            "stderr": stderr,
            "output": output,
            "duration_ms": int((time.monotonic() - started) * 1000),
            "timed_out": timed_out,
            "truncated": stdout_truncated or stderr_truncated,
        }


@dataclass(frozen=True)
class LocalShellToolRegistry:
    driver: "LocalShellDriver"
    tool_name: str = "local_shell"

    @property
    def access_mode(self) -> LocalShellAccessMode:
        return self.driver.access_mode

    def definitions(self) -> list[Tool]:
        return [cast(Tool, dict(local_shell_tool_definition(self.tool_name, **self.driver.presentation_options())))]

    def handlers(self) -> dict[str, Callable[[Mapping[str, Any]], dict[str, Any]]]:
        return {self.tool_name: self.driver.dispatch}

    def execute(self, name: str, args: Mapping[str, Any]) -> dict[str, Any]:
        if name != self.tool_name:
            raise ValueError(f"unknown local shell tool: {name}")
        return self.driver.dispatch(args)

    def requires_approval(self, name: str, args: Mapping[str, Any] | None = None) -> bool:
        return name == self.tool_name and self.driver.requires_approval(args or {})


class LocalShellDriver:
    def __init__(
        self,
        *,
        access_mode: LocalShellAccessMode = "approval",
        runner: LocalCommandRunner | None = None,
        cwd: str | Path | None = None,
        workdir: LocalWorkdir | None = None,
        shell: str | bool | None = None,
        timeout_ms: int = 120_000,
        max_output_bytes: int = 128 * 1024,
    ) -> None:
        root = workdir.root if workdir is not None else cwd
        self.access_mode = access_mode
        self.runner = runner or HostLocalShellRunner(
            cwd=root,
            shell=shell,
            timeout_ms=timeout_ms,
            max_output_bytes=max_output_bytes,
        )

    def dispatch(self, args: Mapping[str, Any]) -> dict[str, Any]:
        request = _shell_request(args)
        if self.access_mode != "full":
            return _shell_approval_required(request)
        return self.runner.run(request)

    def requires_approval(self, args: Mapping[str, Any]) -> bool:
        return self.access_mode != "full"

    def presentation_options(self) -> dict[str, Any]:
        if isinstance(self.runner, HostLocalShellRunner):
            return {
                "access_mode": self.access_mode,
                "cwd": str(self.runner.cwd),
                "shell": self.runner.shell,
                "timeout_ms": self.runner.timeout_ms,
                "max_output_bytes": self.runner.max_output_bytes,
            }
        return {"access_mode": self.access_mode}


def create_local_shell_tool_registry(
    *,
    access_mode: LocalShellAccessMode = "approval",
    tool_name: str = "local_shell",
    runner: LocalCommandRunner | None = None,
    cwd: str | Path | None = None,
    workdir: LocalWorkdir | None = None,
    shell: str | bool | None = None,
    timeout_ms: int = 120_000,
    max_output_bytes: int = 128 * 1024,
) -> LocalShellToolRegistry:
    return LocalShellToolRegistry(
        driver=LocalShellDriver(
            access_mode=access_mode,
            runner=runner,
            cwd=cwd,
            workdir=workdir,
            shell=shell,
            timeout_ms=timeout_ms,
            max_output_bytes=max_output_bytes,
        ),
        tool_name=tool_name,
    )


def local_shell_tool_definition(name: str = "local_shell", **options: Any) -> FunctionTool:
    return {
        "type": "function",
        "name": name,
        "description": local_shell_tool_instructions(**options),
        "parameters": _local_shell_tool_parameters(),
        "strict": False,
    }


def local_shell_tool_instructions(**options: Any) -> str:
    access_mode = options.get("access_mode", "approval")
    cwd = Path(options["cwd"]).resolve() if options.get("cwd") else None
    shell = _shell_display_name(options.get("shell"))
    system = options.get("platform") or platform.system().lower()
    timeout_ms = int(options.get("timeout_ms") or 120_000)
    max_output_bytes = int(options.get("max_output_bytes") or 128 * 1024)
    parts = [
        "Run a local shell command through one model-facing primitive.",
        "Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
        f"Execution environment: platform={system}; shell={shell}; access_mode={access_mode}; default_timeout_ms={timeout_ms}; max_output_bytes={max_output_bytes}.",
        f"Default cwd: {cwd}. Relative command paths resolve from this cwd unless workdir is set." if cwd else "Relative command paths resolve from the configured cwd unless workdir is set.",
        "The workdir parameter must be a relative child path of the configured cwd. Use workdir instead of cd when possible.",
        "Absolute paths inside the command are permitted by the host OS if the user/process has permission; this tool is not a filesystem sandbox or security sandbox.",
        "Captured stdout/stderr may be truncated when output exceeds the advertised max_output_bytes.",
        "In approval mode, calls return requires_approval instead of executing. In full mode, commands execute immediately.",
        _shell_guidance(str(system), options.get("shell")),
    ]
    return " ".join(part for part in parts if part)


def _local_shell_tool_parameters() -> dict[str, Any]:
    return {
        "type": "object",
        "properties": {
            "command": {"type": "string", "description": "Shell command to execute. Keep it focused and quote paths containing spaces."},
            "description": {"type": "string", "description": "Short human-readable description of why this command is being run."},
            "workdir": {
                "type": "string",
                "description": "Optional relative working directory. Use this instead of cd. Absolute workdir values are rejected by the default host runner.",
            },
            "timeout_ms": {"type": "integer", "description": "Optional timeout in milliseconds."},
        },
        "required": ["command"],
        "additionalProperties": False,
    }


def _shell_request(args: Mapping[str, Any]) -> LocalShellRequest:
    return LocalShellRequest(
        command=_required_string(args.get("command"), "command"),
        description=_optional_string(args.get("description"), "description"),
        workdir=_optional_string(args.get("workdir"), "workdir"),
        timeout_ms=_optional_int(args.get("timeout_ms", args.get("timeoutMs")), "timeout_ms"),
    )


def _shell_approval_required(request: LocalShellRequest) -> dict[str, Any]:
    return {
        "ok": False,
        "requires_approval": True,
        "action": "run",
        "command": request.command,
        "description": request.description,
        "workdir": request.workdir,
        "timeout_ms": request.timeout_ms,
        "message": "local_shell command execution requires approval",
    }


def _resolve_contained_path(root: Path, child: str | None) -> Path:
    if not child:
        return root
    raw = Path(child)
    if raw.is_absolute():
        raise ValueError("workdir must be relative to the configured local shell cwd")
    resolved = (root / raw).resolve()
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise ValueError("workdir must stay inside the configured local shell cwd") from exc
    return resolved


def _bounded_decode(value: bytes, max_bytes: int) -> tuple[str, bool]:
    if len(value) <= max_bytes:
        return value.decode("utf-8", errors="replace"), False
    return "...output truncated...\n" + value[-max_bytes:].decode("utf-8", errors="replace"), True


def _shell_display_name(shell: Any) -> str:
    if isinstance(shell, str) and shell.strip():
        return shell
    if shell is False:
        return "direct process execution"
    return "cmd.exe" if os.name == "nt" else "/bin/sh"


def _shell_guidance(system: str, shell: Any) -> str:
    name = Path(shell).name.lower() if isinstance(shell, str) else _shell_display_name(shell).lower()
    if system.startswith("win") or "powershell" in name or name == "pwsh":
        return "On Windows/PowerShell, prefer PowerShell-compatible commands and quote paths with spaces."
    if name in {"cmd", "cmd.exe"}:
        return "On cmd.exe, prefer cmd-compatible syntax and use && only for dependent command chaining."
    return "On POSIX shells, prefer portable shell syntax, quote paths with spaces, and use && only when later commands depend on earlier success."


def _required_string(value: Any, name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} must be a non-empty string")
    return value


def _optional_string(value: Any, name: str) -> str | None:
    if value is None or value == "":
        return None
    if not isinstance(value, str):
        raise ValueError(f"{name} must be a string")
    return value


def _optional_int(value: Any, name: str) -> int | None:
    if value is None:
        return None
    if not isinstance(value, (int, float)) or isinstance(value, bool):
        raise ValueError(f"{name} must be a number")
    return int(value)


def _positive_int(value: int, name: str) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
        raise ValueError(f"{name} must be a positive integer")
    return value
