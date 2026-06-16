from __future__ import annotations

from typing import Any


class ModelsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(self) -> dict[str, Any]:
        return self._http.request("GET", "/v1/models", None)


class PresetsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(self) -> dict[str, Any]:
        return self._http.request("GET", "/v1/presets", None)


class ToolsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(self) -> dict[str, Any]:
        return self._http.request("GET", "/v1/tools", None)


class AsyncModelsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def list(self) -> dict[str, Any]:
        return await self._http.request("GET", "/v1/models", None)


class AsyncPresetsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def list(self) -> dict[str, Any]:
        return await self._http.request("GET", "/v1/presets", None)


class AsyncToolsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def list(self) -> dict[str, Any]:
        return await self._http.request("GET", "/v1/tools", None)
