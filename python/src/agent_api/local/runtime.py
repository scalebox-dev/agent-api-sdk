from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from .config import LocalConfigStore
from .files import LocalFileStore
from .paths import local_app_dirs, normalize_app_name
from .skills import LocalSkillStore
from .types import LocalAppDirs
from .workspace import LocalWorkspace, LocalWorkspaceManager


@dataclass
class LocalRuntime:
    app_name: str
    dirs: LocalAppDirs
    files: LocalFileStore
    data: LocalFileStore
    cache: LocalFileStore
    logs: LocalFileStore
    temp: LocalFileStore
    config: LocalConfigStore
    skills: LocalSkillStore
    workspaces: LocalWorkspaceManager

    def ensure(self) -> None:
        for store in [self.data, self.cache, self.logs, self.temp, self.config.files]:
            store.ensure()

    def workspace(self, root: str | Path, **options: Any) -> LocalWorkspace:
        return LocalWorkspace(root, **options)


def create_local_runtime(
    *,
    app_name: str,
    app_author: str | None = None,
    base_dir: str | Path | None = None,
    dirs: dict[str, str | Path] | None = None,
    env: dict[str, str | None] | None = None,
    platform_name: str | None = None,
) -> LocalRuntime:
    normalized = normalize_app_name(app_name)
    app_dirs = local_app_dirs(
        app_name=normalized,
        app_author=app_author,
        base_dir=base_dir,
        dirs=dirs,
        env=env,
        platform_name=platform_name,
    )
    data = LocalFileStore(app_dirs.data, label="data")
    cache = LocalFileStore(app_dirs.cache, label="cache")
    logs = LocalFileStore(app_dirs.logs, label="logs")
    temp = LocalFileStore(app_dirs.temp, label="temp")
    config_files = LocalFileStore(app_dirs.config, label="config")
    return LocalRuntime(
        app_name=normalized,
        dirs=app_dirs,
        files=data,
        data=data,
        cache=cache,
        logs=logs,
        temp=temp,
        config=LocalConfigStore(config_files),
        skills=LocalSkillStore(data.child("skills")),
        workspaces=LocalWorkspaceManager(),
    )
