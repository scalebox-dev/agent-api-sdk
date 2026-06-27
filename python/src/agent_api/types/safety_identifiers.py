from __future__ import annotations

from typing import Literal, TypedDict


class SafetyIdentifier(TypedDict, total=False):
    object: Literal["safety_identifier"]
    workspace_id: str
    safety_identifier: str
    created_by_user_id: str
    status: str
    created_at: int
    updated_at: int


class ListSafetyIdentifiersResponse(TypedDict, total=False):
    object: Literal["list"]
    data: list[SafetyIdentifier]
