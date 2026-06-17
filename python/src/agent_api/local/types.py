from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Callable, Literal

LocalFileType = Literal["file", "directory", "symlink", "other"]
LocalPathSensitivity = Literal["normal", "sensitive", "secret"]
LocalIgnoreRule = str | re.Pattern[str] | Callable[[str], bool]


@dataclass(frozen=True)
class LocalAppDirs:
    home: str
    data: str
    config: str
    cache: str
    logs: str
    temp: str


@dataclass(frozen=True)
class LocalFileStat:
    path: str
    full_path: str
    type: LocalFileType
    size: int
    modified_at: float
