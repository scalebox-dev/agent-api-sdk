from __future__ import annotations

from pathlib import Path
from typing import Any

from .errors import LocalConfigError
from .files import LocalFileStore


class LocalConfigStore:
    def __init__(self, files: LocalFileStore) -> None:
        self.files = files

    def read(self, name: str = "settings.json", fallback: Any = None) -> Any:
        try:
            return self.files.read_json(name)
        except FileNotFoundError:
            if fallback is not None:
                return fallback
            raise

    def write(self, name: str, value: Any) -> Path:
        return self.files.write_json(name, value)

    def get(self, name: str, key: str, fallback: Any = None) -> Any:
        config = self._read_record(name)
        return config.get(key, fallback)

    def set(self, name: str, key: str, value: Any) -> Path:
        config = self._read_record(name)
        config[key] = value
        return self.write(name, config)

    def delete(self, name: str, key: str) -> Path:
        config = self._read_record(name)
        config.pop(key, None)
        return self.write(name, config)

    def _read_record(self, name: str) -> dict[str, Any]:
        value = self.read(name, {})
        if not isinstance(value, dict):
            raise LocalConfigError(f"local config {name} must contain a JSON object", name)
        return value
