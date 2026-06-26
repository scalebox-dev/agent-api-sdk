from __future__ import annotations

import io
import zipfile
from pathlib import Path
from typing import Any
from urllib.parse import quote, urlencode

from agent_api._utils import build_query, drop_none
from agent_api.types.skills import (
    CreateSkillDevResponse,
    ListSkillFilesResponse,
    ListSkillsResponse,
    ListSkillSummariesResponse,
    Skill,
    SkillFileUpdateMutation,
    SkillBranchDiff,
    SkillFile,
    SkillFileMutation,
    SkillFocusItem,
    SkillFocusResponse,
    SkillArchive,
    SkillDirectoryPullResult,
    SkillImportResponse,
    UpdateSkillFilePrimitiveResponse,
)


class SkillsAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(
        self,
        *,
        include_archived: bool | None = None,
        limit: int | None = None,
        page_token: str | None = None,
        safety_identifier: str | None = None,
    ) -> ListSkillsResponse:
        return self._http.request(
            "GET",
            "/v1/skills" + _safety_query({"include_archived": include_archived, "limit": limit, "page_token": page_token, "safety_identifier": safety_identifier}),
            None,
        )

    def create(self, *, name: str | None = None, description: str | None = None, metadata: dict[str, Any] | None = None, safety_identifier: str | None = None) -> Skill:
        return self._http.request("POST", "/v1/skills", drop_none({"name": name, "description": description, "metadata": metadata, "safety_identifier": safety_identifier}))

    def discover(
        self,
        *,
        query: str | None = None,
        branch: str | None = None,
        include_dev: bool | None = None,
        limit: int | None = None,
        local_skills: list[dict[str, Any]] | None = None,
        safety_identifier: str | None = None,
    ) -> ListSkillSummariesResponse:
        return self._http.request("POST", "/v1/skills/discover", drop_none({"query": query, "branch": branch, "include_dev": include_dev, "limit": limit, "local_skills": local_skills, "safety_identifier": safety_identifier}))

    def focus(
        self,
        *,
        skills: list[SkillFocusItem],
        fallback_to_main: bool | None = None,
        max_manifest_chars: int | None = None,
        max_file_chars: int | None = None,
        safety_identifier: str | None = None,
    ) -> SkillFocusResponse:
        return self._http.request("POST", "/v1/skills/focus", drop_none({"skills": skills, "fallback_to_main": fallback_to_main, "max_manifest_chars": max_manifest_chars, "max_file_chars": max_file_chars, "safety_identifier": safety_identifier}))

    def create_dev(
        self,
        *,
        name: str,
        description: str | None = None,
        metadata: dict[str, Any] | None = None,
        files: list[SkillFileMutation] | None = None,
        safety_identifier: str | None = None,
    ) -> CreateSkillDevResponse:
        return self._http.request("POST", "/v1/skills/create_dev", drop_none({"name": name, "description": description, "metadata": metadata, "files": files, "safety_identifier": safety_identifier}))

    def update_file(self, *, updates: list[SkillFileUpdateMutation], safety_identifier: str | None = None) -> UpdateSkillFilePrimitiveResponse:
        return self._http.request("POST", "/v1/skills/update_file", drop_none({"updates": updates, "safety_identifier": safety_identifier}))

    def retrieve(self, skill_id: str, *, safety_identifier: str | None = None) -> Skill:
        return self._http.request("GET", f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    def update(
        self,
        skill_id: str,
        *,
        name: str | None = None,
        description: str | None = None,
        metadata: dict[str, Any] | None = None,
        safety_identifier: str | None = None,
        new_safety_identifier: str | None = None,
    ) -> Skill:
        body = drop_none({"name": name, "description": description, "metadata": metadata})
        if new_safety_identifier is not None:
            body["safety_identifier"] = new_safety_identifier
        return self._http.request(
            "PATCH",
            f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}, force_safety=new_safety_identifier is not None),
            body,
        )

    def archive(self, skill_id: str, *, safety_identifier: str | None = None) -> Skill:
        return self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/archive" + _safety_query({"safety_identifier": safety_identifier}), {})

    def delete(self, skill_id: str, *, safety_identifier: str | None = None) -> dict[str, bool]:
        return self._http.request("DELETE", f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    def accept_dev(self, skill_id: str, *, strategy: str | None = None) -> Skill:
        return self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/accept_dev" + build_query({"strategy": strategy}), {})

    def discard_dev(self, skill_id: str) -> Skill:
        return self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/discard_dev", {})

    def diff(
        self,
        skill_id: str,
        *,
        path: str | None = None,
        max_file_chars: int | None = None,
        include_unchanged: bool | None = None,
    ) -> SkillBranchDiff:
        return self._http.request(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/diff"
            + build_query({"path": _normalize_archive_path(path) or None, "max_file_chars": max_file_chars, "include_unchanged": include_unchanged}),
            None,
        )

    def list_files(
        self,
        skill_id: str,
        *,
        path: str | None = None,
        branch: str | None = None,
        fallback_to_main: bool | None = None,
        limit: int | None = None,
        page_token: str | None = None,
    ) -> ListSkillFilesResponse:
        return self._http.request(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/files"
            + build_query({"path": path, "branch": branch, "fallback_to_main": fallback_to_main, "limit": limit, "page_token": page_token}),
            None,
        )

    def read_file(self, skill_id: str, path: str, *, branch: str | None = None, fallback_to_main: bool | None = None, max_bytes: int | None = None) -> SkillFile:
        return self._http.request(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}"
            + build_query({"branch": branch, "fallback_to_main": fallback_to_main, "max_bytes": max_bytes}),
            None,
        )

    def write_file(self, skill_id: str, path: str, content: bytes | str, *, branch: str | None = None) -> SkillFile:
        return self._http.request_raw(
            "PUT",
            f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}" + build_query({"branch": branch}),
            content,
        )

    def delete_file(self, skill_id: str, path: str, *, branch: str | None = None) -> SkillFile:
        return self._http.request(
            "DELETE",
            f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}" + build_query({"branch": branch}),
            None,
        )

    def export_archive(
        self,
        skill_id: str,
        *,
        path: str | None = None,
        branch: str | None = None,
        fallback_to_main: bool | None = None,
    ) -> SkillArchive:
        normalized = _normalize_archive_path(path)
        content, headers = self._http.request_binary(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/export"
            + build_query({"path": normalized or None, "branch": branch, "fallback_to_main": fallback_to_main}),
        )
        return {"path": normalized, "content": content, "content_type": headers.get("content-type")}

    def import_archive(
        self,
        skill_id: str,
        archive: bytes,
        *,
        path: str | None = None,
        branch: str | None = None,
        replace: bool | None = None,
        strip_top_level_dir: bool | None = None,
    ) -> SkillImportResponse:
        return self._http.request_raw(
            "POST",
            f"/v1/skills/{quote(skill_id, safe='')}/import"
            + build_query(
                {
                    "path": _normalize_archive_path(path) or None,
                    "branch": branch,
                    "replace": replace,
                    "strip_top_level_dir": strip_top_level_dir,
                }
            ),
            archive,
        )

    def push_directory(
        self,
        skill_id: str,
        root_dir: str | Path,
        *,
        path: str | None = None,
        branch: str | None = None,
        replace: bool | None = None,
        strip_top_level_dir: bool | None = None,
    ) -> SkillImportResponse:
        archive = _zip_directory(root_dir)
        return self.import_archive(
            skill_id,
            archive,
            path=path,
            branch=branch,
            replace=replace,
            strip_top_level_dir=strip_top_level_dir,
        )

    def pull_directory(
        self,
        skill_id: str,
        target_dir: str | Path,
        *,
        path: str | None = None,
        branch: str | None = None,
        fallback_to_main: bool | None = None,
        replace: bool | None = None,
    ) -> SkillDirectoryPullResult:
        archive = self.export_archive(skill_id, path=path, branch=branch, fallback_to_main=fallback_to_main)
        return _extract_zip_to_directory(archive["content"], target_dir, replace=replace is True)


class AsyncSkillsAPI(SkillsAPI):
    async def list(self, *, include_archived: bool | None = None, limit: int | None = None, page_token: str | None = None, safety_identifier: str | None = None) -> ListSkillsResponse:
        return await self._http.request("GET", "/v1/skills" + _safety_query({"include_archived": include_archived, "limit": limit, "page_token": page_token, "safety_identifier": safety_identifier}), None)

    async def create(self, *, name: str | None = None, description: str | None = None, metadata: dict[str, Any] | None = None, safety_identifier: str | None = None) -> Skill:
        return await self._http.request("POST", "/v1/skills", drop_none({"name": name, "description": description, "metadata": metadata, "safety_identifier": safety_identifier}))

    async def discover(self, *, query: str | None = None, branch: str | None = None, include_dev: bool | None = None, limit: int | None = None, local_skills: list[dict[str, Any]] | None = None, safety_identifier: str | None = None) -> ListSkillSummariesResponse:
        return await self._http.request("POST", "/v1/skills/discover", drop_none({"query": query, "branch": branch, "include_dev": include_dev, "limit": limit, "local_skills": local_skills, "safety_identifier": safety_identifier}))

    async def focus(self, *, skills: list[SkillFocusItem], fallback_to_main: bool | None = None, max_manifest_chars: int | None = None, max_file_chars: int | None = None, safety_identifier: str | None = None) -> SkillFocusResponse:
        return await self._http.request("POST", "/v1/skills/focus", drop_none({"skills": skills, "fallback_to_main": fallback_to_main, "max_manifest_chars": max_manifest_chars, "max_file_chars": max_file_chars, "safety_identifier": safety_identifier}))

    async def create_dev(self, *, name: str, description: str | None = None, metadata: dict[str, Any] | None = None, files: list[SkillFileMutation] | None = None, safety_identifier: str | None = None) -> CreateSkillDevResponse:
        return await self._http.request("POST", "/v1/skills/create_dev", drop_none({"name": name, "description": description, "metadata": metadata, "files": files, "safety_identifier": safety_identifier}))

    async def update_file(self, *, updates: list[SkillFileUpdateMutation], safety_identifier: str | None = None) -> UpdateSkillFilePrimitiveResponse:
        return await self._http.request("POST", "/v1/skills/update_file", drop_none({"updates": updates, "safety_identifier": safety_identifier}))

    async def retrieve(self, skill_id: str, *, safety_identifier: str | None = None) -> Skill:
        return await self._http.request("GET", f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    async def update(self, skill_id: str, *, name: str | None = None, description: str | None = None, metadata: dict[str, Any] | None = None, safety_identifier: str | None = None, new_safety_identifier: str | None = None) -> Skill:
        body = drop_none({"name": name, "description": description, "metadata": metadata})
        if new_safety_identifier is not None:
            body["safety_identifier"] = new_safety_identifier
        return await self._http.request("PATCH", f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}, force_safety=new_safety_identifier is not None), body)

    async def archive(self, skill_id: str, *, safety_identifier: str | None = None) -> Skill:
        return await self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/archive" + _safety_query({"safety_identifier": safety_identifier}), {})

    async def delete(self, skill_id: str, *, safety_identifier: str | None = None) -> dict[str, bool]:
        return await self._http.request("DELETE", f"/v1/skills/{quote(skill_id, safe='')}" + _safety_query({"safety_identifier": safety_identifier}), None)

    async def accept_dev(self, skill_id: str, *, strategy: str | None = None) -> Skill:
        return await self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/accept_dev" + build_query({"strategy": strategy}), {})

    async def discard_dev(self, skill_id: str) -> Skill:
        return await self._http.request("POST", f"/v1/skills/{quote(skill_id, safe='')}/discard_dev", {})

    async def diff(self, skill_id: str, *, path: str | None = None, max_file_chars: int | None = None, include_unchanged: bool | None = None) -> SkillBranchDiff:
        return await self._http.request(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/diff"
            + build_query({"path": _normalize_archive_path(path) or None, "max_file_chars": max_file_chars, "include_unchanged": include_unchanged}),
            None,
        )

    async def list_files(self, skill_id: str, *, path: str | None = None, branch: str | None = None, fallback_to_main: bool | None = None, limit: int | None = None, page_token: str | None = None) -> ListSkillFilesResponse:
        return await self._http.request("GET", f"/v1/skills/{quote(skill_id, safe='')}/files" + build_query({"path": path, "branch": branch, "fallback_to_main": fallback_to_main, "limit": limit, "page_token": page_token}), None)

    async def read_file(self, skill_id: str, path: str, *, branch: str | None = None, fallback_to_main: bool | None = None, max_bytes: int | None = None) -> SkillFile:
        return await self._http.request("GET", f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}" + build_query({"branch": branch, "fallback_to_main": fallback_to_main, "max_bytes": max_bytes}), None)

    async def write_file(self, skill_id: str, path: str, content: bytes | str, *, branch: str | None = None) -> SkillFile:
        return await self._http.request_raw("PUT", f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}" + build_query({"branch": branch}), content)

    async def delete_file(self, skill_id: str, path: str, *, branch: str | None = None) -> SkillFile:
        return await self._http.request("DELETE", f"/v1/skills/{quote(skill_id, safe='')}/files/{_skill_path(path)}" + build_query({"branch": branch}), None)

    async def export_archive(
        self,
        skill_id: str,
        *,
        path: str | None = None,
        branch: str | None = None,
        fallback_to_main: bool | None = None,
    ) -> SkillArchive:
        normalized = _normalize_archive_path(path)
        content, headers = await self._http.request_binary(
            "GET",
            f"/v1/skills/{quote(skill_id, safe='')}/export"
            + build_query({"path": normalized or None, "branch": branch, "fallback_to_main": fallback_to_main}),
        )
        return {"path": normalized, "content": content, "content_type": headers.get("content-type")}

    async def import_archive(
        self,
        skill_id: str,
        archive: bytes,
        *,
        path: str | None = None,
        branch: str | None = None,
        replace: bool | None = None,
        strip_top_level_dir: bool | None = None,
    ) -> SkillImportResponse:
        return await self._http.request_raw(
            "POST",
            f"/v1/skills/{quote(skill_id, safe='')}/import"
            + build_query(
                {
                    "path": _normalize_archive_path(path) or None,
                    "branch": branch,
                    "replace": replace,
                    "strip_top_level_dir": strip_top_level_dir,
                }
            ),
            archive,
        )

    async def push_directory(
        self,
        skill_id: str,
        root_dir: str | Path,
        *,
        path: str | None = None,
        branch: str | None = None,
        replace: bool | None = None,
        strip_top_level_dir: bool | None = None,
    ) -> SkillImportResponse:
        archive = _zip_directory(root_dir)
        return await self.import_archive(
            skill_id,
            archive,
            path=path,
            branch=branch,
            replace=replace,
            strip_top_level_dir=strip_top_level_dir,
        )

    async def pull_directory(
        self,
        skill_id: str,
        target_dir: str | Path,
        *,
        path: str | None = None,
        branch: str | None = None,
        fallback_to_main: bool | None = None,
        replace: bool | None = None,
    ) -> SkillDirectoryPullResult:
        archive = await self.export_archive(skill_id, path=path, branch=branch, fallback_to_main=fallback_to_main)
        return _extract_zip_to_directory(archive["content"], target_dir, replace=replace is True)


def _skill_path(path: str) -> str:
    return "/".join(quote(part, safe="") for part in path.split("/") if part)


def _normalize_archive_path(path: str | None) -> str:
    return (path or "").strip().strip("/")


def _safety_query(params: dict[str, Any], *, force_safety: bool = False) -> str:
    filtered = {
        key: value
        for key, value in params.items()
        if value is not None and (value != "" or (force_safety and key == "safety_identifier"))
    }
    return "?" + urlencode(filtered) if filtered else ""


def _zip_directory(root_dir: str | Path) -> bytes:
    root = Path(root_dir).expanduser().resolve()
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", compression=zipfile.ZIP_STORED) as zf:
        for path in sorted(p for p in root.rglob("*") if p.is_file() and ".git" not in p.parts and "__pycache__" not in p.parts and "node_modules" not in p.parts):
            zf.write(path, path.relative_to(root).as_posix())
    return buf.getvalue()


def _extract_zip_to_directory(archive: bytes, target_dir: str | Path, *, replace: bool) -> SkillDirectoryPullResult:
    root = Path(target_dir).expanduser().resolve()
    if replace and root.exists():
        import shutil

        shutil.rmtree(root)
    root.mkdir(parents=True, exist_ok=True)
    file_count = 0
    byte_count = 0
    with zipfile.ZipFile(io.BytesIO(archive), "r") as zf:
        for info in zf.infolist():
            if info.is_dir() or info.filename.startswith("__MACOSX/"):
                continue
            rel = Path(info.filename)
            if rel.is_absolute() or ".." in rel.parts:
                raise ValueError(f"archive entry escapes target directory: {info.filename}")
            target = (root / rel).resolve()
            target.relative_to(root)
            target.parent.mkdir(parents=True, exist_ok=True)
            content = zf.read(info)
            target.write_bytes(content)
            file_count += 1
            byte_count += len(content)
    return {"path": str(root), "file_count": file_count, "byte_count": byte_count}
