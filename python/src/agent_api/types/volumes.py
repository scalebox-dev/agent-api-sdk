from __future__ import annotations

from typing import Literal, TypedDict


class Volume(TypedDict, total=False):
    volume_id: str
    tenant_id: str
    name: str
    oss_prefix: str
    bytes_used: int
    object_count: int
    usage_reconciled_at_unix: int
    created_at_unix: int
    updated_at_unix: int


class VolumeEntry(TypedDict, total=False):
    path: str
    is_dir: bool
    size: int
    modified_at_unix: int


class ListVolumesResponse(TypedDict, total=False):
    object: Literal["list"]
    data: list[Volume]
    next_page_token: str


class ListVolumeEntriesResponse(TypedDict, total=False):
    object: Literal["list"]
    entries: list[VolumeEntry]
    next_page_token: str


class VolumeFileDeliver(TypedDict, total=False):
    path: str
    encoding: Literal["text", "extracted_text", "url", "base64"]
    mime_type: str
    size: int
    truncated: bool
    content: str
    content_base64: str
    image_url: str
    expires_at_unix: int
    extraction_warnings: list[str]


class VolumeFileRaw(TypedDict, total=False):
    path: str
    size: int
    truncated: bool
    content: bytes
    content_type: str


class VolumeFileWrite(TypedDict, total=False):
    path: str
    size: int


class VolumePathDelete(TypedDict, total=False):
    path: str
    recursive: bool


class VolumeFileLines(TypedDict, total=False):
    path: str
    start_line: int
    end_line: int
    total_lines: int
    lines: list[str]
    file_truncated: bool
    size: int


class VolumeFileLinesPatch(TypedDict, total=False):
    path: str
    start_line: int
    end_line: int
    total_lines: int
    size: int


class VolumeSummaryPreview(TypedDict, total=False):
    path: str
    size: int
    preview: str
    preview_truncated: bool


class VolumeSummary(TypedDict, total=False):
    summary_path: str
    file_count: int
    total_bytes: int
    top_paths_by_size: list[str]
    text_previews: list[VolumeSummaryPreview]
    generated_at_unix: int


class VolumeGrepMatch(TypedDict, total=False):
    path: str
    line_number: int
    line: str


class VolumeGrepResponse(TypedDict, total=False):
    object: Literal["list"]
    matches: list[VolumeGrepMatch]
    next_page_token: str
    files_scanned: int
    scan_truncated: bool


class VolumeArchive(TypedDict, total=False):
    path: str
    content: bytes
    content_type: str
