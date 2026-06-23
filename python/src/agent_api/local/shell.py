from __future__ import annotations

import os
import platform
import json
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
LocalShellIsolationMode = Literal["none", "auto", "required"]


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
        isolation_options: Mapping[str, Any] | None = None,
    ) -> None:
        self.cwd = Path(cwd or os.getcwd()).resolve()
        self.shell = shell if shell is not None else True
        self.timeout_ms = _positive_int(timeout_ms, "timeout_ms")
        self.max_output_bytes = _positive_int(max_output_bytes, "max_output_bytes")
        self.isolation_status = _direct_isolation_status(False, isolation_options)
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
            "shell_isolation": self.isolation_status,
        }


class IsolatorLocalShellRunner:
    def __init__(
        self,
        *,
        executable_path: str | Path | None = None,
        driver: str = "auto",
        cwd: str | Path | None = None,
        timeout_ms: int = 120_000,
        max_output_bytes: int = 128 * 1024,
        isolation_options: Mapping[str, Any] | None = None,
    ) -> None:
        self.executable_path = _resolve_isolator_executable_path(executable_path)
        self.driver = driver or "auto"
        self.cwd = Path(cwd or os.getcwd()).resolve()
        self.timeout_ms = _positive_int(timeout_ms, "timeout_ms")
        self.max_output_bytes = _positive_int(max_output_bytes, "max_output_bytes")
        self.isolation_options = dict(isolation_options or {})
        self.status_result = _isolator_status_sync(self.executable_path, self.driver)
        self.isolation_status = self.status_result["status"]

    def run(self, request: LocalShellRequest) -> dict[str, Any]:
        command = _required_string(request.command, "command")
        cwd = _resolve_contained_path(self.cwd, request.workdir)
        timeout_ms = self.timeout_ms if request.timeout_ms is None else _positive_int(request.timeout_ms, "timeout_ms")
        response = _isolator_request(
            self.executable_path,
            self.driver,
            {
                "id": f"run_{int(time.time() * 1000)}",
                "method": "run",
                "params": {
                    "command": command,
                    "description": request.description,
                    "cwd": str(cwd),
                    "timeout_ms": timeout_ms,
                    "max_output_bytes": self.max_output_bytes,
                    "env": dict(request.env or {}),
                    "isolation": self.isolation_options,
                },
            },
        )
        return _isolator_run_result(response.get("result"))


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
        isolation: LocalShellIsolationMode = "none",
        isolation_options: Mapping[str, Any] | None = None,
        runner: LocalCommandRunner | None = None,
        cwd: str | Path | None = None,
        workdir: LocalWorkdir | None = None,
        shell: str | bool | None = None,
        timeout_ms: int = 120_000,
        max_output_bytes: int = 128 * 1024,
        isolator: bool | Mapping[str, Any] = False,
    ) -> None:
        root = workdir.root if workdir is not None else cwd
        self.access_mode = access_mode
        self.isolation_options = isolation_options
        self.runner = _resolve_shell_runner(
            isolation=isolation,
            runner=runner,
            cwd=root,
            shell=shell,
            timeout_ms=timeout_ms,
            max_output_bytes=max_output_bytes,
            isolation_options=isolation_options,
            isolator=isolator,
        )
        self.isolation_status = _runner_isolation_status(self.runner) or _direct_isolation_status(isolation == "auto", isolation_options)

    def dispatch(self, args: Mapping[str, Any]) -> dict[str, Any]:
        request = _shell_request(args)
        if self.access_mode != "full":
            return _shell_approval_required(request, self.isolation_status)
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
                "isolation_status": self.isolation_status,
                "isolation_options": self.isolation_options,
            }
        return {"access_mode": self.access_mode, "isolation_status": self.isolation_status, "isolation_options": self.isolation_options}


def create_local_shell_tool_registry(
    *,
    access_mode: LocalShellAccessMode = "approval",
    isolation: LocalShellIsolationMode = "none",
    isolation_options: Mapping[str, Any] | None = None,
    tool_name: str = "local_shell",
    runner: LocalCommandRunner | None = None,
    cwd: str | Path | None = None,
    workdir: LocalWorkdir | None = None,
    shell: str | bool | None = None,
    timeout_ms: int = 120_000,
    max_output_bytes: int = 128 * 1024,
    isolator: bool | Mapping[str, Any] = False,
) -> LocalShellToolRegistry:
    return LocalShellToolRegistry(
        driver=LocalShellDriver(
            access_mode=access_mode,
            isolation=isolation,
            isolation_options=isolation_options,
            runner=runner,
            cwd=cwd,
            workdir=workdir,
            shell=shell,
            timeout_ms=timeout_ms,
            max_output_bytes=max_output_bytes,
            isolator=isolator,
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
    isolation = options.get("isolation_status") or _direct_isolation_status(False, options.get("isolation_options"))
    cwd = Path(options["cwd"]).resolve() if options.get("cwd") else None
    shell = _shell_display_name(options.get("shell"))
    system = options.get("platform") or platform.system().lower()
    timeout_ms = int(options.get("timeout_ms") or 120_000)
    max_output_bytes = int(options.get("max_output_bytes") or 128 * 1024)
    parts = [
        "Run a local shell command through one model-facing primitive.",
        "Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
        f"Execution environment: platform={system}; shell={shell}; access_mode={access_mode}; isolation_driver={isolation.get('driver')}; isolated={str(isolation.get('isolated')).lower()}; fallback={str(isolation.get('fallback')).lower()}; default_timeout_ms={timeout_ms}; max_output_bytes={max_output_bytes}.",
        _isolation_request_description(cast(Mapping[str, Any], isolation.get("requested") or _normalize_isolation_options(None))),
        f"Isolation warning: {' '.join(isolation.get('warnings') or [])}" if isolation.get("warnings") else "",
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


def _shell_approval_required(request: LocalShellRequest, isolation_status: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "ok": False,
        "requires_approval": True,
        "action": "run",
        "command": request.command,
        "description": request.description,
        "workdir": request.workdir,
        "timeout_ms": request.timeout_ms,
        "shell_isolation": dict(isolation_status),
        "message": "local_shell command execution requires approval",
    }


def _resolve_shell_runner(
    *,
    isolation: LocalShellIsolationMode,
    runner: LocalCommandRunner | None,
    cwd: str | Path | None,
    shell: str | bool | None,
    timeout_ms: int,
    max_output_bytes: int,
    isolation_options: Mapping[str, Any] | None,
    isolator: bool | Mapping[str, Any],
) -> LocalCommandRunner:
    if runner is not None:
        status = _runner_isolation_status(runner)
        if isolation == "required" and not (status and status.get("isolated") is True):
            raise ValueError("local_shell isolation is required, but the configured runner does not report isolation")
        return runner
    isolator_fallback_warning: str | None = None
    if isolation in {"auto", "required"} or isolator:
        options = dict(isolator) if isinstance(isolator, Mapping) else {}
        try:
            isolator_runner = IsolatorLocalShellRunner(
                cwd=cwd,
                timeout_ms=timeout_ms,
                max_output_bytes=max_output_bytes,
                isolation_options=isolation_options,
                **options,
            )
            status = _runner_isolation_status(isolator_runner)
            if isolation == "required" and not (status and status.get("isolated") is True):
                raise ValueError(
                    f"local_shell isolation is required, but agent-isolator selected non-isolated driver {status.get('driver') if status else 'unknown'}"
                )
            if isolation != "none" or isolator:
                return isolator_runner
        except Exception as exc:
            if isolation == "required":
                raise
            isolator_fallback_warning = str(exc)
    if isolation == "required":
        raise ValueError("local_shell isolation is required, but no isolating runner was configured")
    host = HostLocalShellRunner(cwd=cwd, shell=shell, timeout_ms=timeout_ms, max_output_bytes=max_output_bytes, isolation_options=isolation_options)
    if isolation == "auto":
        host.isolation_status = _direct_isolation_status(True, isolation_options)
        if isolator_fallback_warning:
            host.isolation_status = {
                **host.isolation_status,
                "warnings": [
                    *cast(list[str], host.isolation_status.get("warnings") or []),
                    f"Isolator unavailable: {isolator_fallback_warning}",
                ],
            }
    return host


def _runner_isolation_status(runner: LocalCommandRunner) -> Mapping[str, Any] | None:
    status = getattr(runner, "isolation_status", None)
    return status if isinstance(status, Mapping) else None


def _resolve_isolator_executable_path(executable_path: str | Path | None) -> str:
    configured = str(executable_path).strip() if executable_path is not None else ""
    configured = configured or os.environ.get("AGENT_ISOLATOR_PATH", "").strip()
    if not configured:
        raise RuntimeError("agent-isolator executable path is not configured; pass isolator.executable_path or set AGENT_ISOLATOR_PATH")
    return configured


def _isolator_status_sync(executable_path: str, driver: str) -> dict[str, Any]:
    completed = subprocess.run(
        [executable_path, "--once", f"--driver={driver}"],
        input=json.dumps({"id": "status", "method": "status", "params": {}}).encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout).decode("utf-8", errors="replace").strip()
        raise RuntimeError(details or f"agent-isolator exited with status {completed.returncode}")
    envelope = _parse_isolator_envelope(completed.stdout.decode("utf-8", errors="replace"))
    if envelope.get("error"):
        error = cast(Mapping[str, Any], envelope["error"])
        raise RuntimeError(str(error.get("message") or error.get("code") or "agent-isolator status failed"))
    return _isolator_status_result(envelope.get("result"))


def _isolator_request(executable_path: str, driver: str, envelope: Mapping[str, Any]) -> dict[str, Any]:
    completed = subprocess.run(
        [executable_path, "--once", f"--driver={driver}"],
        input=json.dumps(dict(envelope)).encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout).decode("utf-8", errors="replace").strip()
        raise RuntimeError(details or f"agent-isolator exited with status {completed.returncode}")
    parsed = _parse_isolator_envelope(completed.stdout.decode("utf-8", errors="replace"))
    if parsed.get("error"):
        error = cast(Mapping[str, Any], parsed["error"])
        raise RuntimeError(str(error.get("message") or error.get("code") or "agent-isolator request failed"))
    return parsed


def _parse_isolator_envelope(text: str) -> dict[str, Any]:
    trimmed = text.strip()
    if not trimmed:
        raise RuntimeError("agent-isolator returned an empty response")
    value = json.loads(trimmed)
    if not isinstance(value, dict):
        raise RuntimeError("agent-isolator response must be an object")
    return value


def _isolator_status_result(value: Any) -> dict[str, Any]:
    if not isinstance(value, Mapping):
        raise RuntimeError("agent-isolator status result must be an object")
    return {
        "version": value.get("version") if isinstance(value.get("version"), str) else None,
        "driver": _required_string(value.get("driver"), "driver"),
        "status": _isolation_status_from_unknown(value.get("status")),
        "drivers": [
            {
                "name": str(item.get("name", "")),
                "platform": str(item.get("platform", "")),
                "available": item.get("available") is True,
                "warnings": [str(warning) for warning in item.get("warnings", [])] if isinstance(item.get("warnings"), list) else None,
            }
            for item in value.get("drivers", [])
            if isinstance(item, Mapping)
        ]
        if isinstance(value.get("drivers"), list)
        else None,
    }


def _isolator_run_result(value: Any) -> dict[str, Any]:
    if not isinstance(value, Mapping):
        raise RuntimeError("agent-isolator run result must be an object")
    return {
        "ok": value.get("ok") is True,
        "action": "run",
        "command": _required_string(value.get("command"), "command"),
        "description": value.get("description") if isinstance(value.get("description"), str) else None,
        "cwd": _required_string(value.get("cwd"), "cwd"),
        "exit_code": value.get("exit_code") if isinstance(value.get("exit_code"), int) else None,
        "signal": value.get("signal") if isinstance(value.get("signal"), str) else None,
        "stdout": value.get("stdout") if isinstance(value.get("stdout"), str) else "",
        "stderr": value.get("stderr") if isinstance(value.get("stderr"), str) else "",
        "output": value.get("output") if isinstance(value.get("output"), str) else "(no output)",
        "duration_ms": value.get("duration_ms") if isinstance(value.get("duration_ms"), int | float) else 0,
        "timed_out": value.get("timed_out") is True,
        "truncated": value.get("truncated") is True,
        "shell_isolation": _isolation_status_from_unknown(value.get("shell_isolation")),
    }


def _isolation_status_from_unknown(value: Any) -> dict[str, Any]:
    if not isinstance(value, Mapping):
        raise RuntimeError("shell isolation status must be an object")
    return {
        "executor": "isolator" if value.get("executor") == "isolator" else "direct",
        "driver": _required_string(value.get("driver"), "shell_isolation.driver"),
        "isolated": value.get("isolated") is True,
        "fallback": value.get("fallback") is True,
        "requested": _normalize_isolation_options(value.get("requested") if isinstance(value.get("requested"), Mapping) else None),
        "guarantees": _isolation_guarantees_from_unknown(value.get("guarantees")),
        "warnings": [str(warning) for warning in value.get("warnings", [])] if isinstance(value.get("warnings"), list) else [],
    }


def _isolation_guarantees_from_unknown(value: Any) -> dict[str, Any]:
    record = value if isinstance(value, Mapping) else {}
    return {
        "filesystem": record.get("filesystem") if record.get("filesystem") in {"workdir-mounted", "policy-enforced"} else "none",
        "network": record.get("network") if record.get("network") in {"blocked", "configurable"} else "allowed",
        "user": record.get("user") if record.get("user") in {"unprivileged-user", "namespace-user"} else "host-user",
        "process": record.get("process") if record.get("process") in {"child-contained", "pid-namespace"} else "host-process-tree",
        "resources": record.get("resources") if record.get("resources") in {"cpu-memory-limits", "timeout-only"} else "none",
    }


def _direct_isolation_status(fallback: bool, options: Mapping[str, Any] | None = None) -> dict[str, Any]:
    requested = _normalize_isolation_options(options)
    return {
        "executor": "direct",
        "driver": "direct",
        "isolated": False,
        "fallback": fallback,
        "requested": requested,
        "guarantees": {
            "filesystem": "none",
            "network": "allowed",
            "user": "host-user",
            "process": "host-process-tree",
            "resources": "timeout-only",
        },
        "warnings": _direct_isolation_warnings(fallback, requested),
    }


def _normalize_isolation_options(options: Mapping[str, Any] | None) -> dict[str, Any]:
    resources = options.get("resources") if isinstance(options, Mapping) and isinstance(options.get("resources"), Mapping) else {}
    return {
        "filesystem": options.get("filesystem", "host") if isinstance(options, Mapping) else "host",
        "network": options.get("network", "allowed") if isinstance(options, Mapping) else "allowed",
        "env": options.get("env", "inherit") if isinstance(options, Mapping) else "inherit",
        "resources": {
            "memoryMb": resources.get("memoryMb"),
            "cpuCount": resources.get("cpuCount"),
        },
    }


def _direct_isolation_warnings(fallback: bool, requested: Mapping[str, Any]) -> list[str]:
    warnings = [
        "No local shell isolator is configured; falling back to direct host execution."
        if fallback
        else "Direct host execution has no OS-level isolation."
    ]
    if requested.get("filesystem") != "host":
        warnings.append(f"Requested filesystem isolation ({requested.get('filesystem')}) is not enforced by direct execution.")
    if requested.get("network") == "blocked":
        warnings.append("Requested network blocking is not enforced by direct execution.")
    if requested.get("env") == "minimal":
        warnings.append("Requested minimal environment is not enforced by direct execution.")
    resources = requested.get("resources") or {}
    if isinstance(resources, Mapping) and (resources.get("memoryMb") is not None or resources.get("cpuCount") is not None):
        warnings.append("Requested CPU or memory limits are not enforced by direct execution.")
    return warnings


def _isolation_request_description(options: Mapping[str, Any]) -> str:
    resources = options.get("resources") or {}
    resource_parts = []
    if isinstance(resources, Mapping):
        if resources.get("memoryMb") is not None:
            resource_parts.append(f"memory_mb={resources.get('memoryMb')}")
        if resources.get("cpuCount") is not None:
            resource_parts.append(f"cpu_count={resources.get('cpuCount')}")
    suffix = f"; {'; '.join(resource_parts)}" if resource_parts else ""
    return f"Requested isolation: filesystem={options.get('filesystem')}; network={options.get('network')}; env={options.get('env')}{suffix}."


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
