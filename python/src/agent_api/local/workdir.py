from __future__ import annotations

import hashlib
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .errors import LocalError, LocalFileTooLargeError, LocalIgnoredPathError, LocalNotTextFileError
from .files import LocalFileStore
from .paths import (
    classify_local_path_sensitivity,
    default_workdir_ignore_rules,
    entry_from_stat,
    ignored,
    line_edit_params,
    looks_binary,
    normalize_relative_path,
    parse_ignore_file,
    patch_line_range,
    positive_int,
    snapshot_file_changed,
    split_lines,
)
from .types import LocalFileStat, LocalIgnoreRule


class LocalWorkdirManager:
    def open(self, root: str | Path, **options: Any) -> "LocalWorkdir":
        return LocalWorkdir(root, **options)


@dataclass
class LocalWorkdir:
    root: Path | str
    name: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    trusted: bool = False
    ignore: list[LocalIgnoreRule] = field(default_factory=list)
    gitignore: bool = True
    max_file_bytes: int = 10 * 1024 * 1024

    def __post_init__(self) -> None:
        self.root = Path(self.root).expanduser().resolve()
        self.name = (self.name or self.root.name or "workdir").strip()
        self.files = LocalFileStore(self.root, label="workdir")
        self.ignore = [*default_workdir_ignore_rules(), *self.ignore]

    def ensure(self) -> None:
        self.files.ensure()
        if self.gitignore:
            self.load_ignore_files()

    def load_ignore_files(self, files: list[str] | None = None) -> list[LocalIgnoreRule]:
        loaded: list[LocalIgnoreRule] = []
        for file in files or [".gitignore"]:
            try:
                loaded.extend(parse_ignore_file(self.files.read_text(file)))
            except FileNotFoundError:
                continue
        self.ignore.extend(loaded)
        return loaded

    def child(self, relative_path: str | Path, **options: Any) -> "LocalWorkdir":
        return LocalWorkdir(
            self.files.resolve_path(relative_path),
            name=options.get("name"),
            metadata=options.get("metadata", self.metadata),
            trusted=options.get("trusted", self.trusted),
            ignore=options.get("ignore", self.ignore),
            gitignore=options.get("gitignore", self.gitignore),
            max_file_bytes=options.get("max_file_bytes", self.max_file_bytes),
        )

    def resolve_path(self, relative_path: str | Path = ".") -> Path:
        self._assert_allowed(relative_path)
        return self.files.resolve_path(relative_path)

    def list(self, relative_path: str | Path = ".", **options: Any) -> list[LocalFileStat]:
        options["ignore"] = self._merge_ignore(options.get("ignore"))
        return self.files.list(relative_path, **options)

    def list_entries(self, relative_path: str | Path = ".", **options: Any) -> dict[str, Any]:
        options["ignore"] = self._merge_ignore(options.get("ignore"))
        return self.files.list_entries(relative_path, **options)

    def search_entries(self, **params: Any) -> dict[str, Any]:
        query = str(params.get("query", "")).strip().lower()
        if not query:
            raise ValueError("query is required")
        limit = positive_int(params.get("limit"), 100)
        path = params.get("path", ".")
        return {
            "object": "list",
            "entries": [
                entry_from_stat(item)
                for item in self.list(path, recursive=True, include_directories=True)
                if query in item.path.lower()
            ][:limit],
        }

    def read_file(self, relative_path: str | Path, **params: Any) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        params.setdefault("max_bytes", self.max_file_bytes)
        return self.files.read_file(relative_path, **params)

    def write_file(self, relative_path: str | Path, content: str | bytes, **options: Any) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        return self.files.write_file(relative_path, content, **options)

    def read_text(self, relative_path: str | Path) -> str:
        self._assert_allowed(relative_path)
        return self.files.read_text(relative_path)

    def write_text(self, relative_path: str | Path, content: str, **options: Any) -> Path:
        self._assert_allowed(relative_path)
        return self.files.write_text(relative_path, content, **options)

    def delete_path(self, relative_path: str | Path) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        return self.files.delete_path(relative_path)

    def create_directory(self, relative_path: str | Path = ".") -> dict[str, Any]:
        self._assert_allowed(relative_path)
        return self.files.create_directory(relative_path)

    def read_lines(self, relative_path: str | Path, **params: Any) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        params.setdefault("max_bytes", self.max_file_bytes)
        return self.files.read_lines(relative_path, **params)

    def preview_patch_lines(self, relative_path: str | Path, **params: Any) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        lines = self.read_lines(
            relative_path,
            start_line=params["start_line"],
            end_line=params.get("end_line"),
            max_bytes=params.get("max_bytes", self.max_file_bytes),
        )
        file = self.files.read_file(relative_path, max_bytes=params.get("max_bytes", self.max_file_bytes), format="raw")
        if file["truncated"]:
            raise LocalFileTooLargeError(str(relative_path))
        if looks_binary(file["content"]):
            raise LocalNotTextFileError(str(relative_path))
        _, _, total = patch_line_range(file["content"].decode("utf-8"), params["start_line"], params.get("end_line"), params.get("replacement", ""))
        return {
            "path": lines["path"],
            "start_line": lines["start_line"],
            "end_line": lines["end_line"],
            "total_lines": total,
            "before": lines["lines"],
            "after": [] if params.get("replacement", "") == "" else split_lines(params.get("replacement", "")),
        }

    def patch_lines(self, relative_path: str | Path, **params: Any) -> dict[str, Any]:
        self._assert_allowed(relative_path)
        params.setdefault("max_bytes", self.max_file_bytes)
        return self.files.patch_lines(relative_path, **params)

    def preview_edits(self, edits: list[dict[str, Any]]) -> dict[str, Any]:
        previews = []
        for edit in edits:
            self._assert_expected_hash(edit["path"], edit.get("expected_sha256"))
            previews.append(self.preview_patch_lines(edit["path"], **line_edit_params(edit)))
        return {"edits": [dict(edit) for edit in edits], "previews": previews}

    def apply_edits(self, edits: list[dict[str, Any]]) -> dict[str, Any]:
        backups: list[dict[str, str]] = []
        applied: list[dict[str, Any]] = []
        try:
            for edit in edits:
                self._assert_expected_hash(edit["path"], edit.get("expected_sha256"))
                backups.append({"path": edit["path"], "content": self.read_text(edit["path"])})
                applied.append(self.patch_lines(edit["path"], **line_edit_params(edit)))
        except Exception:
            for backup in reversed(backups):
                self.write_text(backup["path"], backup["content"])
            raise
        return {"applied": applied, "backups": backups}

    def classify_path(self, relative_path: str | Path) -> dict[str, Any]:
        return classify_local_path_sensitivity(relative_path)

    def grep(self, **params: Any) -> dict[str, Any]:
        params["ignore"] = self._merge_ignore(params.get("ignore"))
        return self.files.grep(**params)

    def summarize(self, **params: Any) -> dict[str, Any]:
        params["ignore"] = self._merge_ignore(params.get("ignore"))
        return self.files.summarize(**params)

    def snapshot(self, *, path: str | Path = ".", hash: bool = True, max_bytes_per_file: int | None = None) -> dict[str, Any]:
        max_bytes = positive_int(max_bytes_per_file, self.max_file_bytes)
        files = []
        for item in self.list(path, recursive=True):
            if item.type != "file":
                continue
            snap: dict[str, Any] = {"path": item.path, "size": item.size, "modified_at_unix": int(item.modified_at)}
            if hash and item.size <= max_bytes:
                snap["sha256"] = hashlib.sha256(Path(item.full_path).read_bytes()).hexdigest()
            files.append(snap)
        return {"root": str(self.root), "name": self.name, "generated_at_unix": int(time.time()), "files": files}

    def diff(self, before: dict[str, Any], after: dict[str, Any]) -> dict[str, Any]:
        before_by_path = {file["path"]: file for file in before.get("files", [])}
        after_by_path = {file["path"]: file for file in after.get("files", [])}
        added = []
        modified = []
        unchanged = []
        for after_file in after_by_path.values():
            before_file = before_by_path.get(after_file["path"])
            if before_file is None:
                added.append(after_file)
            elif snapshot_file_changed(before_file, after_file):
                modified.append({"before": before_file, "after": after_file})
            else:
                unchanged.append(after_file)
        deleted = [file for path, file in before_by_path.items() if path not in after_by_path]
        return {"added": added, "modified": modified, "deleted": deleted, "unchanged": unchanged}

    def _merge_ignore(self, ignore: list[LocalIgnoreRule] | None) -> list[LocalIgnoreRule]:
        return [*self.ignore, *(ignore or [])]

    def _assert_allowed(self, relative_path: str | Path) -> None:
        rel = normalize_relative_path(relative_path)
        if rel != "." and ignored(rel, self.ignore):
            raise LocalIgnoredPathError(rel)

    def _assert_expected_hash(self, relative_path: str | Path, expected_sha256: str | None) -> None:
        if not expected_sha256:
            return
        actual = hashlib.sha256(self.files.resolve_path(relative_path).read_bytes()).hexdigest()
        if actual != expected_sha256:
            raise LocalError("local_edit_conflict", f"local file changed before edit: {relative_path}", path=str(relative_path))
