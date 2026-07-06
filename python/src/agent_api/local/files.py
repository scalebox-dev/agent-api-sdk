from __future__ import annotations

import base64
import errno
import shutil
import time
from pathlib import Path
from typing import Any, Literal

from .errors import LocalFileTooLargeError, LocalNotTextFileError, LocalPathError
from .paths import (
    assert_inside_root,
    atomic_write,
    entry_from_stat,
    file_type,
    ignored,
    is_likely_text_file,
    looks_binary,
    mime_type,
    normalize_relative_path,
    patch_line_range,
    portable_path,
    positive_int,
    select_line_range,
    split_lines,
)
from .types import LocalFileStat, LocalIgnoreRule


class LocalFileStore:
    def __init__(self, root: str | Path, *, label: str = "local") -> None:
        self.root = Path(root).expanduser().resolve()
        self.label = label

    def child(self, relative_path: str | Path, *, label: str | None = None) -> "LocalFileStore":
        return LocalFileStore(self.resolve_path(relative_path), label=label or self.label)

    def ensure(self) -> None:
        self.root.mkdir(parents=True, exist_ok=True)

    def resolve_path(self, relative_path: str | Path = ".") -> Path:
        clean = normalize_relative_path(relative_path)
        full_path = (self.root / clean).resolve()
        assert_inside_root(self.root, full_path)
        return full_path

    def relative_path(self, full_path: str | Path) -> str:
        absolute = Path(full_path).expanduser().resolve()
        assert_inside_root(self.root, absolute)
        return portable_path(absolute.relative_to(self.root))

    def exists(self, relative_path: str | Path = ".") -> bool:
        return self.resolve_path(relative_path).exists()

    def stat(self, relative_path: str | Path = ".") -> LocalFileStat:
        full_path = self.resolve_path(relative_path)
        info = full_path.lstat()
        return LocalFileStat(
            path=portable_path(full_path.relative_to(self.root)) or ".",
            full_path=str(full_path),
            type=file_type(full_path, info),
            size=info.st_size,
            modified_at=info.st_mtime,
        )

    def mkdir(self, relative_path: str | Path = ".") -> Path:
        full_path = self.resolve_path(relative_path)
        full_path.mkdir(parents=True, exist_ok=True)
        return full_path

    def list(
        self,
        relative_path: str | Path = ".",
        *,
        recursive: bool = False,
        include_directories: bool = False,
        max_depth: int | None = None,
        ignore: list[LocalIgnoreRule] | None = None,
    ) -> list[LocalFileStat]:
        base = self.resolve_path(relative_path)
        depth_limit = max_depth if recursive else 1
        out: list[LocalFileStat] = []
        self._walk(self.root, base, base, out, depth_limit, include_directories, ignore or [])
        return sorted(out, key=lambda item: item.path)

    def list_entries(self, relative_path: str | Path = ".", **options: Any) -> dict[str, Any]:
        options.setdefault("include_directories", True)
        return {"object": "list", "entries": [entry_from_stat(item) for item in self.list(relative_path, **options)]}

    def search_entries(self, *, query: str, path: str | Path = ".", limit: int = 100) -> dict[str, Any]:
        needle = query.strip().lower()
        if not needle:
            raise ValueError("query is required")
        entries = [
            entry_from_stat(item)
            for item in self.list(path, recursive=True, include_directories=True)
            if needle in item.path.lower()
        ][: positive_int(limit, 100)]
        return {"object": "list", "entries": entries}

    def read_text(self, relative_path: str | Path) -> str:
        return self.resolve_path(relative_path).read_text(encoding="utf-8")

    def read_json(self, relative_path: str | Path) -> Any:
        import json

        return json.loads(self.read_text(relative_path))

    def read_bytes(self, relative_path: str | Path) -> bytes:
        return self.resolve_path(relative_path).read_bytes()

    def read_file(self, relative_path: str | Path, *, max_bytes: int | None = None, format: Literal["text", "raw"] = "text") -> dict[str, Any]:
        full_path = self.resolve_path(relative_path)
        if not full_path.is_file():
            raise LocalPathError("local path is not a file", str(relative_path))
        raw = full_path.read_bytes()
        limit = max_bytes if max_bytes is not None else len(raw)
        truncated = len(raw) > limit
        content = raw[:limit] if truncated else raw
        portable = portable_path(full_path.relative_to(self.root))
        if format == "raw":
            return {"path": portable, "size": len(raw), "truncated": truncated, "content": content, "content_type": mime_type(portable)}
        if looks_binary(content):
            return {
                "path": portable,
                "encoding": "base64",
                "mime_type": mime_type(portable),
                "size": len(raw),
                "truncated": truncated,
                "content_base64": base64.b64encode(content).decode("ascii"),
            }
        return {
            "path": portable,
            "encoding": "text",
            "mime_type": mime_type(portable),
            "size": len(raw),
            "truncated": truncated,
            "content": content.decode("utf-8"),
        }

    def write_text(self, relative_path: str | Path, content: str, *, atomic: bool = True) -> Path:
        full_path = self.resolve_path(relative_path)
        full_path.parent.mkdir(parents=True, exist_ok=True)
        if atomic:
            atomic_write(full_path, content.encode("utf-8"))
        else:
            full_path.write_text(content, encoding="utf-8")
        return full_path

    def write_json(self, relative_path: str | Path, value: Any, *, pretty: bool = True, atomic: bool = True) -> Path:
        import json

        indent = 2 if pretty else None
        return self.write_text(relative_path, json.dumps(value, indent=indent) + "\n", atomic=atomic)

    def write_bytes(self, relative_path: str | Path, content: bytes, *, atomic: bool = True) -> Path:
        full_path = self.resolve_path(relative_path)
        full_path.parent.mkdir(parents=True, exist_ok=True)
        if atomic:
            atomic_write(full_path, content)
        else:
            full_path.write_bytes(content)
        return full_path

    def write_file(self, relative_path: str | Path, content: str | bytes, *, atomic: bool = True) -> dict[str, Any]:
        full_path = self.write_text(relative_path, content, atomic=atomic) if isinstance(content, str) else self.write_bytes(relative_path, content, atomic=atomic)
        return {"path": portable_path(full_path.relative_to(self.root)), "size": full_path.stat().st_size}

    def remove(self, relative_path: str | Path) -> None:
        full_path = self.resolve_path(relative_path)
        if full_path.is_dir():
            shutil.rmtree(full_path, ignore_errors=True)
        else:
            full_path.unlink(missing_ok=True)

    def delete_path(self, relative_path: str | Path) -> dict[str, Any]:
        self.remove(relative_path)
        return {"path": normalize_relative_path(relative_path), "recursive": True}

    def create_directory(self, relative_path: str | Path = ".") -> dict[str, Any]:
        self.mkdir(relative_path)
        return {"path": normalize_relative_path(relative_path)}

    def copy(self, from_relative_path: str | Path, to_relative_path: str | Path) -> Path:
        source = self.resolve_path(from_relative_path)
        target = self.resolve_path(to_relative_path)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        return target

    def read_lines(self, relative_path: str | Path, *, start_line: int, end_line: int | None = None, max_bytes: int | None = None) -> dict[str, Any]:
        file = self.read_file(relative_path, max_bytes=max_bytes, format="raw")
        if looks_binary(file["content"]):
            raise LocalNotTextFileError(str(relative_path))
        text = file["content"].decode("utf-8")
        lines = split_lines(text)
        selected, resolved_end = select_line_range(lines, start_line, end_line)
        return {
            "path": file["path"],
            "start_line": int(start_line),
            "end_line": resolved_end,
            "total_lines": len(lines),
            "lines": selected,
            "file_truncated": file["truncated"],
            "size": file["size"],
        }

    def patch_lines(
        self,
        relative_path: str | Path,
        *,
        start_line: int,
        end_line: int | None = None,
        replacement: str = "",
        max_bytes: int | None = None,
    ) -> dict[str, Any]:
        file = self.read_file(relative_path, max_bytes=max_bytes, format="raw")
        if file["truncated"]:
            raise LocalFileTooLargeError(str(relative_path))
        if looks_binary(file["content"]):
            raise LocalNotTextFileError(str(relative_path))
        patched, resolved_end, total = patch_line_range(file["content"].decode("utf-8"), start_line, end_line, replacement)
        written = self.write_file(relative_path, patched)
        return {"path": file["path"], "start_line": int(start_line), "end_line": resolved_end, "total_lines": total, "size": written["size"]}

    def grep(
        self,
        *,
        pattern: str,
        path: str | Path = ".",
        limit: int = 200,
        max_files: int = 500,
        max_bytes_per_file: int = 512 * 1024,
        max_line_length: int = 500,
        ignore: list[LocalIgnoreRule] | None = None,
    ) -> dict[str, Any]:
        needle = pattern.strip()
        if not needle:
            raise ValueError("pattern is required")
        matches: list[dict[str, Any]] = []
        files_scanned = 0
        scan_truncated = False
        for item in self._grep_candidates(path, ignore or []):
            if len(matches) >= limit or files_scanned >= max_files:
                scan_truncated = True
                break
            if item.type != "file" or item.size > max_bytes_per_file or not is_likely_text_file(item.path):
                continue
            raw = read_optional_bytes(Path(item.full_path))
            if raw is None:
                continue
            if looks_binary(raw):
                continue
            files_scanned += 1
            for idx, line in enumerate(split_lines(raw.decode("utf-8")), start=1):
                if needle not in line:
                    continue
                matches.append({"path": item.path, "line_number": idx, "line": line[:max_line_length]})
                if len(matches) >= limit:
                    scan_truncated = True
                    break
        return {"object": "list", "matches": matches, "files_scanned": files_scanned, "scan_truncated": scan_truncated}

    def _grep_candidates(self, relative_path: str | Path, ignore: list[LocalIgnoreRule]) -> list[LocalFileStat]:
        full_path = self.resolve_path(relative_path)
        if not full_path.exists():
            raise FileNotFoundError(errno.ENOENT, "No such file or directory", str(full_path))
        portable = portable_path(full_path.relative_to(self.root)) or "."
        if ignored(portable, ignore):
            return []
        info = full_path.lstat()
        if full_path.is_file():
            return [LocalFileStat(portable, str(full_path), file_type(full_path, info), info.st_size, info.st_mtime)]
        if full_path.is_dir():
            return self.list(relative_path, recursive=True, ignore=ignore)
        return []

    def summarize(
        self,
        *,
        path: str | Path = ".",
        max_files: int = 2000,
        max_previews: int = 20,
        preview_bytes: int = 4096,
        top_paths: int = 20,
        max_depth: int | None = None,
        ignore: list[LocalIgnoreRule] | None = None,
    ) -> dict[str, Any]:
        stats = self.list(path, recursive=True, max_depth=max_depth, ignore=ignore)
        all_files = [item for item in stats if item.type == "file"]
        files = all_files[:max_files]
        by_size = sorted(files, key=lambda item: (-item.size, item.path))
        previews: list[dict[str, Any]] = []
        for item in by_size:
            if len(previews) >= max_previews:
                break
            if not is_likely_text_file(item.path) or item.size > preview_bytes * 4:
                continue
            raw = read_optional_bytes(Path(item.full_path))
            if raw is None:
                continue
            if looks_binary(raw):
                continue
            previews.append(
                {
                    "path": item.path,
                    "size": item.size,
                    "preview": raw[:preview_bytes].decode("utf-8"),
                    "preview_truncated": len(raw) > preview_bytes or None,
                }
            )
        return {
            "summary_path": "",
            "file_count": len(files),
            "total_bytes": sum(item.size for item in files),
            "top_paths_by_size": [f"{item.path} ({item.size} bytes)" for item in by_size[:top_paths]],
            "text_previews": previews,
            "generated_at_unix": int(time.time()),
            "scan_truncated": len(all_files) > len(files),
        }

    def _walk(
        self,
        store_root: Path,
        scan_root: Path,
        directory: Path,
        out: list[LocalFileStat],
        max_depth: int | None,
        include_directories: bool,
        ignore: list[LocalIgnoreRule],
    ) -> None:
        if not directory.exists():
            return
        for entry in sorted(directory.iterdir(), key=lambda item: item.name):
            rel = portable_path(entry.absolute().relative_to(store_root))
            if ignored(rel, ignore):
                continue
            info = lstat_optional(entry)
            if info is None:
                continue
            stat = LocalFileStat(rel, str(entry), file_type(entry, info), info.st_size, info.st_mtime)
            if entry.is_dir() and not entry.is_symlink():
                if include_directories:
                    out.append(stat)
                depth = len(portable_path(entry.absolute().relative_to(scan_root)).split("/"))
                if max_depth is None or depth < max_depth:
                    self._walk(store_root, scan_root, entry, out, max_depth, include_directories, ignore)
            else:
                out.append(stat)


def lstat_optional(path: Path) -> Any | None:
    try:
        return path.lstat()
    except OSError as exc:
        if exc.errno in {errno.ENOENT, errno.ENOTDIR}:
            return None
        raise


def read_optional_bytes(path: Path) -> bytes | None:
    try:
        return path.read_bytes()
    except OSError as exc:
        if exc.errno in {errno.ENOENT, errno.ENOTDIR, errno.EISDIR}:
            return None
        raise
