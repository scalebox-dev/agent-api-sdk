from __future__ import annotations

from pathlib import Path
from typing import Any

from agent_api.local_skills import local_skill_from_directory

from .files import LocalFileStore
from .paths import discover_skill_directories


class LocalSkillStore:
    def __init__(self, files: LocalFileStore) -> None:
        self.files = files

    def from_directory(self, root_dir: str | Path, **options: Any) -> dict[str, Any]:
        return local_skill_from_directory(root_dir, **options)

    def discover(
        self,
        *,
        roots: list[str | Path] | None = None,
        recursive: bool = False,
        max_depth: int | None = None,
        **options: Any,
    ) -> list[dict[str, Any]]:
        scan_roots = [Path(root).expanduser().resolve() for root in (roots or [self.files.root])]
        dirs: set[Path] = set()
        for root in scan_roots:
            dirs.update(discover_skill_directories(root, recursive=recursive, max_depth=max_depth))
        return [local_skill_from_directory(path, **options) for path in sorted(dirs)]
