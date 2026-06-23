from __future__ import annotations

import fnmatch
import mimetypes
import os
import platform
import re
import stat as stat_module
import tempfile
import uuid
from pathlib import Path
from typing import Any

from .errors import LocalConfigError, LocalPathError
from .types import LocalAppDirs, LocalFileStat, LocalFileType, LocalIgnoreRule, LocalPathSensitivity


def local_app_dirs(
    *,
    app_name: str,
    app_author: str | None = None,
    base_dir: str | Path | None = None,
    dirs: dict[str, str | Path] | None = None,
    env: dict[str, str | None] | None = None,
    platform_name: str | None = None,
) -> LocalAppDirs:
    normalized = normalize_app_name(app_name)
    env_map = env if env is not None else dict(os.environ)
    system = platform_name or platform.system().lower()
    home = Path(env_map.get("HOME") or env_map.get("USERPROFILE") or Path.home()).expanduser().resolve()
    author_segment = sanitize_path_segment(app_author or normalized)
    app_segment = sanitize_path_segment(normalized)
    overrides = dirs or {}

    if base_dir is not None:
        base = Path(base_dir).expanduser().resolve()
        defaults = {
            "data": base / "data",
            "config": base / "config",
            "cache": base / "cache",
            "logs": base / "logs",
            "temp": base / "tmp",
        }
    elif system == "darwin":
        defaults = {
            "data": home / "Library" / "Application Support" / app_segment,
            "config": home / "Library" / "Application Support" / app_segment,
            "cache": home / "Library" / "Caches" / app_segment,
            "logs": home / "Library" / "Logs" / app_segment,
            "temp": Path(tempfile.gettempdir()) / app_segment,
        }
    elif system in {"windows", "win32"}:
        roaming = Path(env_map.get("APPDATA") or home / "AppData" / "Roaming")
        local = Path(env_map.get("LOCALAPPDATA") or home / "AppData" / "Local")
        defaults = {
            "data": roaming / author_segment / app_segment,
            "config": roaming / author_segment / app_segment,
            "cache": local / author_segment / app_segment / "Cache",
            "logs": local / author_segment / app_segment / "Logs",
            "temp": Path(tempfile.gettempdir()) / app_segment,
        }
    else:
        defaults = {
            "data": Path(env_map.get("XDG_DATA_HOME") or home / ".local" / "share") / app_segment,
            "config": Path(env_map.get("XDG_CONFIG_HOME") or home / ".config") / app_segment,
            "cache": Path(env_map.get("XDG_CACHE_HOME") or home / ".cache") / app_segment,
            "logs": Path(env_map.get("XDG_STATE_HOME") or home / ".local" / "state") / app_segment / "logs",
            "temp": Path(tempfile.gettempdir()) / app_segment,
        }

    return LocalAppDirs(
        home=str(home),
        data=str(Path(overrides.get("data", defaults["data"])).expanduser().resolve()),
        config=str(Path(overrides.get("config", defaults["config"])).expanduser().resolve()),
        cache=str(Path(overrides.get("cache", defaults["cache"])).expanduser().resolve()),
        logs=str(Path(overrides.get("logs", defaults["logs"])).expanduser().resolve()),
        temp=str(Path(overrides.get("temp", defaults["temp"])).expanduser().resolve()),
    )


def normalize_app_name(app_name: str) -> str:
    trimmed = app_name.strip()
    if not trimmed:
        raise LocalConfigError("app_name is required")
    return trimmed


def sanitize_path_segment(value: str) -> str:
    return re.sub(r"[^a-z0-9._-]+", "-", value.strip().lower()).strip("-") or "agent-api"


def normalize_relative_path(value: str | Path) -> str:
    raw = str(value).strip()
    if not raw or raw == ".":
        return "."
    path = Path(raw)
    if path.is_absolute():
        raise LocalPathError("local path must be relative", raw)
    return raw.replace("\\", "/")


def assert_inside_root(root: Path, full_path: Path) -> None:
    try:
        full_path.relative_to(root)
    except ValueError as exc:
        raise LocalPathError("local path must stay inside the store root", str(full_path)) from exc


def portable_path(value: Path) -> str:
    return value.as_posix()


def file_type(path: Path, stat: os.stat_result | None = None) -> LocalFileType:
    if stat is not None:
        mode = stat.st_mode
        if stat_module.S_ISLNK(mode):
            return "symlink"
        if stat_module.S_ISREG(mode):
            return "file"
        if stat_module.S_ISDIR(mode):
            return "directory"
        return "other"
    if path.is_symlink():
        return "symlink"
    if path.is_file():
        return "file"
    if path.is_dir():
        return "directory"
    return "other"


def entry_from_stat(item: LocalFileStat) -> dict[str, Any]:
    return {"path": item.path, "is_dir": item.type == "directory", "size": 0 if item.type == "directory" else item.size, "modified_at_unix": int(item.modified_at)}


def atomic_write(full_path: Path, content: bytes) -> None:
    tmp_path = full_path.parent / f".{full_path.name}.{os.getpid()}.{uuid.uuid4().hex}.tmp"
    tmp_path.write_bytes(content)
    tmp_path.replace(full_path)


def ignored(relative_path: str, ignore: list[LocalIgnoreRule]) -> bool:
    for rule in ignore:
        if isinstance(rule, str):
            clean = rule.strip().replace("\\", "/").strip("/")
            if not clean:
                continue
            if "*" in clean:
                if fnmatch.fnmatch(relative_path, clean) or fnmatch.fnmatch(Path(relative_path).name, clean):
                    return True
            elif relative_path == clean or relative_path.startswith(f"{clean}/") or relative_path.endswith(f"/{clean}") or f"/{clean}/" in relative_path:
                return True
        elif hasattr(rule, "search"):
            if rule.search(relative_path):
                return True
        elif rule(relative_path):
            return True
    return False


def default_workdir_ignore_rules() -> list[LocalIgnoreRule]:
    return [".git", "node_modules", "__pycache__", ".DS_Store", "dist", "build", "coverage", ".next", ".turbo", ".cache", re.compile(r"\.pyc$"), re.compile(r"\.pyo$"), re.compile(r"\.class$"), re.compile(r"\.log$")]


def parse_ignore_file(text: str) -> list[LocalIgnoreRule]:
    rules: list[LocalIgnoreRule] = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or line.startswith("!"):
            continue
        line = line.replace("\\", "/").lstrip("/").rstrip("/")
        rules.append(line)
    return rules


def discover_skill_directories(root: Path, *, recursive: bool, max_depth: int | None) -> set[Path]:
    if not root.is_dir():
        return set()
    found: set[Path] = set()
    depth_limit = max_depth if recursive else 1

    def walk(directory: Path, depth: int) -> None:
        if (directory / "SKILL.md").is_file():
            found.add(directory)
        if depth_limit is not None and depth >= depth_limit:
            return
        for child in directory.iterdir():
            if child.is_dir() and child.name not in {".git", "node_modules", "__pycache__"}:
                walk(child, depth + 1)

    walk(root, 0)
    return found


def positive_int(value: Any, fallback: int) -> int:
    try:
        parsed = int(value)
        return parsed if parsed > 0 else fallback
    except (TypeError, ValueError):
        return fallback


def looks_binary(content: bytes) -> bool:
    return b"\0" in content


TEXT_EXTENSIONS = {
    ".txt", ".md", ".markdown", ".json", ".yaml", ".yml", ".toml", ".xml", ".csv", ".py", ".go", ".js", ".mjs",
    ".cjs", ".ts", ".tsx", ".jsx", ".html", ".htm", ".css", ".scss", ".sh", ".bash", ".zsh", ".sql", ".env",
    ".ini", ".cfg", ".conf", ".log", ".rst", ".adoc", ".gradle", ".properties", ".mod", ".sum", ".dockerfile",
}


def is_likely_text_file(relative_path: str) -> bool:
    lower = relative_path.lower()
    suffix = Path(lower).suffix
    return not suffix or suffix in TEXT_EXTENSIONS or lower.endswith("dockerfile")


def mime_type(relative_path: str) -> str:
    guessed, _ = mimetypes.guess_type(relative_path)
    if guessed:
        return guessed
    return "text/plain" if is_likely_text_file(relative_path) else "application/octet-stream"


def split_lines(content: str) -> list[str]:
    if content == "":
        return [""]
    if content.endswith("\n"):
        content = content[:-1]
        if content == "":
            return [""]
    return content.split("\n")


def select_line_range(lines: list[str], start_line: int, end_line: int | None) -> tuple[list[str], int]:
    start = int(start_line)
    if start < 1:
        raise ValueError("start_line must be >= 1")
    total = len(lines) or 1
    end = total if end_line is None or int(end_line) <= 0 else min(int(end_line), total)
    if start > total or end < start:
        raise ValueError("invalid line range")
    return lines[start - 1 : end], end


def patch_line_range(content: str, start_line: int, end_line: int | None, replacement: str) -> tuple[str, int, int]:
    lines = split_lines(content)
    _, resolved_end = select_line_range(lines, start_line, end_line)
    replacement_lines = [] if replacement == "" else split_lines(replacement)
    patched = [*lines[: int(start_line) - 1], *replacement_lines, *lines[resolved_end:]]
    out = "\n".join(patched)
    if content.endswith("\n"):
        out += "\n"
    return out, resolved_end, len(patched)


def classify_local_path_sensitivity(relative_path: str | Path) -> dict[str, Any]:
    rel = normalize_relative_path(relative_path)
    lower = rel.lower()
    base = Path(lower).name
    if base == ".env" or base.startswith(".env.") or "id_rsa" in lower or "id_ed25519" in lower or lower.endswith((".pem", ".key", ".p12", ".pfx")):
        return {"path": rel, "sensitivity": "secret", "reason": "path commonly contains credentials or private keys"}
    if any(token in lower for token in ["secret", "token", "credential", "password"]) or lower.endswith((".crt", ".cert")):
        return {"path": rel, "sensitivity": "sensitive", "reason": "path name suggests sensitive material"}
    return {"path": rel, "sensitivity": "normal"}


def summarize_local_context_sensitivity(files: list[dict[str, Any]]) -> dict[str, Any]:
    order = {"normal": 0, "sensitive": 1, "secret": 2}
    highest: LocalPathSensitivity = "normal"
    sensitive_files = []
    for file in files:
        sensitivity = file.get("sensitivity", "normal")
        if order.get(sensitivity, 0) > order[highest]:
            highest = sensitivity
        if sensitivity != "normal":
            sensitive_files.append({"path": file.get("path"), "sensitivity": sensitivity, "reason": file.get("sensitivity_reason")})
    return {"highest": highest, "files": sensitive_files}


def matches_path_filters(relative_path: str, include: list[str] | None, exclude: list[str] | None) -> bool:
    if include and not any(path_matches(relative_path, pattern) for pattern in include):
        return False
    if exclude and any(path_matches(relative_path, pattern) for pattern in exclude):
        return False
    return True


def path_matches(relative_path: str, pattern: str) -> bool:
    clean = pattern.replace("\\", "/").lstrip("/")
    if not clean or clean == ".":
        return True
    if "*" not in clean:
        return relative_path == clean or relative_path.startswith(f"{clean}/")
    return fnmatch.fnmatch(relative_path, clean)


def snapshot_file_changed(before: dict[str, Any], after: dict[str, Any]) -> bool:
    if before.get("sha256") or after.get("sha256"):
        return before.get("sha256") != after.get("sha256")
    return before.get("size") != after.get("size") or before.get("modified_at_unix") != after.get("modified_at_unix")


def line_edit_params(edit: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in edit.items() if key not in {"path", "expected_sha256"}}
