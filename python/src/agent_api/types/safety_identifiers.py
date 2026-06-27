from __future__ import annotations

from typing import Literal, TypedDict


class SafetyIdentifierPartition(TypedDict, total=False):
    object: Literal["safety_identifier"]
    workspace_id: str
    safety_identifier: str
    owner_user_id: str
    status: str
    created_at: int
    updated_at: int


class ListSafetyIdentifierPartitionsResponse(TypedDict, total=False):
    object: Literal["list"]
    data: list[SafetyIdentifierPartition]
