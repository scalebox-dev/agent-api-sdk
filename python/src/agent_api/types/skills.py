from __future__ import annotations

from typing import Any, Literal, TypedDict


class SkillReference(TypedDict, total=False):
    skill_id: str
    branch: str


class LocalSkillDescriptor(TypedDict, total=False):
    local_skill_id: str
    skill_ref: str
    name: str
    description: str
    root_hint: str
    digest: str
    manifest: str
    manifest_truncated: bool
    metadata: dict[str, Any]


class Skill(TypedDict, total=False):
    object: Literal["skill"]
    skill_id: str
    tenant_id: str
    name: str
    description: str
    source_type: str
    main_digest: str
    dev_digest: str
    has_dev: bool
    dev_updated_at: int
    dev_source_response_id: str
    created_by_user_id: str
    updated_by_user_id: str
    created_at: int
    updated_at: int
    metadata: dict[str, Any]
    archived: bool


class SkillSummary(TypedDict, total=False):
    object: Literal["skill_summary"]
    skill_id: str
    skill_ref: str
    name: str
    description: str
    source_type: str
    branch: str
    digest: str
    artifact_uri: str
    has_dev: bool
    metadata: dict[str, Any]


class FocusedSkill(SkillSummary, total=False):
    object: Literal["focused_skill"]
    mount_hint: str
    manifest: str
    manifest_truncated: bool
    entries: list["SkillFileEntry"]
    files: list["SkillFocusedFile"]


class SkillFocusedFile(TypedDict, total=False):
    path: str
    content: str
    truncated: bool
    size: int
    branch: str
    error: "SkillOperationError"


class SkillFocusItem(TypedDict, total=False):
    skill_id: str
    branch: str
    paths: list[str]
    include_manifest: bool
    local_skill: LocalSkillDescriptor


class SkillOperationError(TypedDict, total=False):
    code: str
    message: str


class SkillFocusResultItem(TypedDict, total=False):
    ok: bool
    skill_ref: str
    skill_id: str
    local_skill_id: str
    branch: str
    skill: FocusedSkill
    error: SkillOperationError


class SkillFocusResponse(TypedDict, total=False):
    object: Literal["skill_focus_result"]
    data: list[SkillFocusResultItem]


class SkillFileEntry(TypedDict, total=False):
    path: str
    is_dir: bool
    size: int
    modified_at: int


class ListSkillsResponse(TypedDict, total=False):
    object: Literal["list"]
    data: list[Skill]
    next_page_token: str


class ListSkillSummariesResponse(TypedDict, total=False):
    object: Literal["list"]
    data: list[SkillSummary]
    next_page_token: str


class SkillFileMutation(TypedDict, total=False):
    path: str
    content: str
    content_base64: str


class SkillFileUpdateMutation(SkillFileMutation, total=False):
    skill_id: str


class CreateSkillDevResponse(TypedDict, total=False):
    object: Literal["skill_create_result"]
    skill: Skill
    branch: str
    files: list[dict[str, Any]]
    focused_skill: FocusedSkill


class SkillUpdateResultItem(TypedDict, total=False):
    ok: bool
    skill_id: str
    path: str
    size: int
    skill: SkillSummary
    error: SkillOperationError


class UpdateSkillFilePrimitiveResponse(TypedDict, total=False):
    object: Literal["skill_update_result"]
    data: list[SkillUpdateResultItem]


class ListSkillFilesResponse(TypedDict, total=False):
    object: Literal["list"]
    entries: list[SkillFileEntry]
    next_page_token: str


class SkillFile(TypedDict, total=False):
    object: Literal["skill_file"]
    path: str
    branch: str
    content: str
    size: int
    truncated: bool
    recursive: bool


class SkillArchive(TypedDict, total=False):
    path: str
    content: bytes
    content_type: str


class SkillImportResponse(TypedDict, total=False):
    object: Literal["skill_import_result"]
    branch: str
    file_count: int
    byte_count: int
    skill: Skill


class SkillBranchDiffFile(TypedDict, total=False):
    path: str
    status: str
    base_size: int
    compare_size: int
    text: bool
    binary: bool
    too_large: bool
    truncated: bool
    diff: str


class SkillBranchDiff(TypedDict, total=False):
    object: Literal["skill_branch_diff"]
    skill_id: str
    base_branch: str
    compare_branch: str
    path: str
    summary: dict[str, int]
    files: list[SkillBranchDiffFile]


class SkillDirectoryPullResult(TypedDict, total=False):
    path: str
    file_count: int
    byte_count: int
