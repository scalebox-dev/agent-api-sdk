from __future__ import annotations

from typing import Any, Literal, overload
from urllib.parse import quote, urlencode

from agent_api._utils import build_query, drop_none
from agent_api.types.volumes import (
    ListVolumeEntriesResponse,
    ListVolumesResponse,
    Volume,
    VolumeArchive,
    VolumeFileDeliver,
    VolumeFileLines,
    VolumeFileLinesPatch,
    VolumeFileRaw,
    VolumeFileWrite,
    VolumeGrepResponse,
    VolumePathDelete,
    VolumeSummary,
)


class VolumesAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(
        self,
        *,
        limit: int | None = None,
        page_token: str | None = None,
        safety_identifier: str | None = None,
        user_id: str | None = None,
    ) -> ListVolumesResponse:
        return self._http.request(
            "GET",
            "/v1/volumes"
            + _safety_query(
                {"limit": limit, "page_token": page_token, "safety_identifier": safety_identifier, "user_id": user_id}
            ),
            None,
        )

    def create(self, *, name: str | None = None, safety_identifier: str | None = None) -> Volume:
        return self._http.request("POST", "/v1/volumes", drop_none({"name": name, "safety_identifier": safety_identifier}))

    def retrieve(self, volume_id: str, *, safety_identifier: str | None = None) -> Volume:
        return self._http.request("GET", f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    def update(
        self,
        volume_id: str,
        *,
        name: str | None = None,
        safety_identifier: str | None = None,
        new_safety_identifier: str | None = None,
    ) -> Volume:
        body = drop_none({"name": name})
        if new_safety_identifier is not None:
            body["safety_identifier"] = new_safety_identifier
        return self._http.request(
            "PATCH",
            f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}, force_safety=new_safety_identifier is not None),
            body,
        )

    def delete(self, volume_id: str, *, safety_identifier: str | None = None) -> None:
        self._http.request_void("DELETE", f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}))

    def list_entries(
        self,
        volume_id: str,
        *,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> ListVolumeEntriesResponse:
        return self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/entries"
            + build_query({"path": path, "limit": limit, "page_token": page_token}),
            None,
        )

    def search_entries(
        self,
        volume_id: str,
        *,
        query: str,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> ListVolumeEntriesResponse:
        return self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/search"
            + build_query({"query": query, "path": path, "limit": limit, "page_token": page_token}),
            None,
        )

    @overload
    def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        format: Literal["raw"],
        max_bytes: int | None = None,
    ) -> VolumeFileRaw: ...

    @overload
    def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        max_bytes: int | None = None,
        format: None = None,
    ) -> VolumeFileDeliver: ...

    def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        max_bytes: int | None = None,
        format: Literal["raw"] | None = None,
    ) -> VolumeFileDeliver | VolumeFileRaw:
        url = (
            f"/v1/volumes/{quote(volume_id, safe='')}/files/{_volume_path(path)}"
            + build_query({"max_bytes": max_bytes, "format": format})
        )
        if format == "raw":
            content, headers = self._http.request_binary("GET", url)
            size_header = headers.get("X-Volume-Size")
            return {
                "path": path,
                "size": int(size_header) if size_header else len(content),
                "truncated": headers.get("X-Volume-Truncated") == "true",
                "content": content,
                "content_type": headers.get("content-type"),
            }
        return self._http.request("GET", url, None)

    def write_file(self, volume_id: str, path: str, content: bytes | str) -> VolumeFileWrite:
        return self._http.request_raw("PUT", f"/v1/volumes/{quote(volume_id, safe='')}/files/{_volume_path(path)}", content)

    def delete_path(self, volume_id: str, path: str) -> VolumePathDelete:
        return self._http.request("DELETE", f"/v1/volumes/{quote(volume_id, safe='')}/paths/{_volume_path(path)}", None)

    def reconcile_usage(self, volume_id: str) -> Volume:
        return self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/usage/reconcile",
            None,
        )

    def create_directory(self, volume_id: str, path: str) -> dict[str, str]:
        return self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/directories",
            {"path": path},
        )

    def download_archive(self, volume_id: str, *, path: str | None = None) -> VolumeArchive:
        normalized = _normalize_archive_path(path)
        url = (
            f"/v1/volumes/{quote(volume_id, safe='')}/archive"
            + build_query({"path": normalized or None})
        )
        content, headers = self._http.request_binary("GET", url)
        return {
            "path": normalized,
            "content": content,
            "content_type": headers.get("content-type"),
        }

    def summarize(self, volume_id: str, *, path: str | None = None) -> VolumeSummary:
        return self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/summarize",
            drop_none({"path": path}),
        )

    def read_lines(
        self,
        volume_id: str,
        path: str,
        *,
        start_line: int,
        end_line: int | None = None,
        max_bytes: int | None = None,
    ) -> VolumeFileLines:
        return self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/file_lines/{_volume_path(path)}"
            + build_query({"start_line": start_line, "end_line": end_line, "max_bytes": max_bytes}),
            None,
        )

    def patch_lines(
        self,
        volume_id: str,
        path: str,
        *,
        start_line: int,
        end_line: int | None = None,
        replacement: str = "",
    ) -> VolumeFileLinesPatch:
        return self._http.request(
            "PATCH",
            f"/v1/volumes/{quote(volume_id, safe='')}/file_lines/{_volume_path(path)}",
            drop_none({"start_line": start_line, "end_line": end_line, "replacement": replacement}),
        )

    def grep(
        self,
        volume_id: str,
        *,
        pattern: str,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> VolumeGrepResponse:
        return self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/grep"
            + build_query({"pattern": pattern, "path": path, "limit": limit, "page_token": page_token}),
            None,
        )


class AsyncVolumesAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def list(
        self,
        *,
        limit: int | None = None,
        page_token: str | None = None,
        safety_identifier: str | None = None,
        user_id: str | None = None,
    ) -> ListVolumesResponse:
        return await self._http.request(
            "GET",
            "/v1/volumes"
            + _safety_query(
                {"limit": limit, "page_token": page_token, "safety_identifier": safety_identifier, "user_id": user_id}
            ),
            None,
        )

    async def create(self, *, name: str | None = None, safety_identifier: str | None = None) -> Volume:
        return await self._http.request("POST", "/v1/volumes", drop_none({"name": name, "safety_identifier": safety_identifier}))

    async def retrieve(self, volume_id: str, *, safety_identifier: str | None = None) -> Volume:
        return await self._http.request("GET", f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    async def update(
        self,
        volume_id: str,
        *,
        name: str | None = None,
        safety_identifier: str | None = None,
        new_safety_identifier: str | None = None,
    ) -> Volume:
        body = drop_none({"name": name})
        if new_safety_identifier is not None:
            body["safety_identifier"] = new_safety_identifier
        return await self._http.request(
            "PATCH",
            f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}, force_safety=new_safety_identifier is not None),
            body,
        )

    async def delete(self, volume_id: str, *, safety_identifier: str | None = None) -> None:
        await self._http.request_void("DELETE", f"/v1/volumes/{quote(volume_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}))

    async def list_entries(
        self,
        volume_id: str,
        *,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> ListVolumeEntriesResponse:
        return await self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/entries"
            + build_query({"path": path, "limit": limit, "page_token": page_token}),
            None,
        )

    async def search_entries(
        self,
        volume_id: str,
        *,
        query: str,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> ListVolumeEntriesResponse:
        return await self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/search"
            + build_query({"query": query, "path": path, "limit": limit, "page_token": page_token}),
            None,
        )

    @overload
    async def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        format: Literal["raw"],
        max_bytes: int | None = None,
    ) -> VolumeFileRaw: ...

    @overload
    async def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        max_bytes: int | None = None,
        format: None = None,
    ) -> VolumeFileDeliver: ...

    async def read_file(
        self,
        volume_id: str,
        path: str,
        *,
        max_bytes: int | None = None,
        format: Literal["raw"] | None = None,
    ) -> VolumeFileDeliver | VolumeFileRaw:
        url = (
            f"/v1/volumes/{quote(volume_id, safe='')}/files/{_volume_path(path)}"
            + build_query({"max_bytes": max_bytes, "format": format})
        )
        if format == "raw":
            content, headers = await self._http.request_binary("GET", url)
            size_header = headers.get("X-Volume-Size")
            return {
                "path": path,
                "size": int(size_header) if size_header else len(content),
                "truncated": headers.get("X-Volume-Truncated") == "true",
                "content": content,
                "content_type": headers.get("content-type"),
            }
        return await self._http.request("GET", url, None)

    async def write_file(self, volume_id: str, path: str, content: bytes | str) -> VolumeFileWrite:
        return await self._http.request_raw(
            "PUT",
            f"/v1/volumes/{quote(volume_id, safe='')}/files/{_volume_path(path)}",
            content,
        )

    async def delete_path(self, volume_id: str, path: str) -> VolumePathDelete:
        return await self._http.request(
            "DELETE",
            f"/v1/volumes/{quote(volume_id, safe='')}/paths/{_volume_path(path)}",
            None,
        )

    async def reconcile_usage(self, volume_id: str) -> Volume:
        return await self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/usage/reconcile",
            None,
        )

    async def create_directory(self, volume_id: str, path: str) -> dict[str, str]:
        return await self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/directories",
            {"path": path},
        )

    async def download_archive(self, volume_id: str, *, path: str | None = None) -> VolumeArchive:
        normalized = _normalize_archive_path(path)
        url = (
            f"/v1/volumes/{quote(volume_id, safe='')}/archive"
            + build_query({"path": normalized or None})
        )
        content, headers = await self._http.request_binary("GET", url)
        return {
            "path": normalized,
            "content": content,
            "content_type": headers.get("content-type"),
        }

    async def summarize(self, volume_id: str, *, path: str | None = None) -> VolumeSummary:
        return await self._http.request(
            "POST",
            f"/v1/volumes/{quote(volume_id, safe='')}/summarize",
            drop_none({"path": path}),
        )

    async def read_lines(
        self,
        volume_id: str,
        path: str,
        *,
        start_line: int,
        end_line: int | None = None,
        max_bytes: int | None = None,
    ) -> VolumeFileLines:
        return await self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/file_lines/{_volume_path(path)}"
            + build_query({"start_line": start_line, "end_line": end_line, "max_bytes": max_bytes}),
            None,
        )

    async def patch_lines(
        self,
        volume_id: str,
        path: str,
        *,
        start_line: int,
        end_line: int | None = None,
        replacement: str = "",
    ) -> VolumeFileLinesPatch:
        return await self._http.request(
            "PATCH",
            f"/v1/volumes/{quote(volume_id, safe='')}/file_lines/{_volume_path(path)}",
            drop_none({"start_line": start_line, "end_line": end_line, "replacement": replacement}),
        )

    async def grep(
        self,
        volume_id: str,
        *,
        pattern: str,
        path: str | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> VolumeGrepResponse:
        return await self._http.request(
            "GET",
            f"/v1/volumes/{quote(volume_id, safe='')}/grep"
            + build_query({"pattern": pattern, "path": path, "limit": limit, "page_token": page_token}),
            None,
        )


def _normalize_archive_path(path: str | None) -> str:
    if not path:
        return ""
    return path.strip().strip("/").replace("//", "/")


def _volume_path(path: str) -> str:
    return "/".join(quote(part, safe="") for part in path.split("/") if part)


def _safety_query(params: dict[str, Any], *, force_safety: bool = False) -> str:
    filtered = {
        key: value
        for key, value in params.items()
        if value is not None and (value != "" or (force_safety and key == "safety_identifier"))
    }
    return "?" + urlencode(filtered) if filtered else ""
