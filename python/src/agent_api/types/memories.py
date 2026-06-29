from __future__ import annotations

from typing import Any, Literal, TypedDict

from agent_api.types.common import Usage


class MemorySearchParams(TypedDict, total=False):
    query: str
    limit: int
    previous_response_id: str
    tenant_search: bool
    lang: str
    semantic_weight: float


class MemorySearchHit(TypedDict, total=False):
    id: str
    score: float
    thread_id: str
    created_at: int
    fact: str
    tenant_id: str
    user_id: str
    response_id: str
    metadata: Any
    metadata_text: str


class MemorySearchModelUsage(TypedDict, total=False):
    source: str
    phase: str
    provider: str
    model: str
    attempt_index: int
    status: str
    usage: Usage


class MemorySearchResponse(TypedDict, total=False):
    object: Literal["memory_search_result"]
    data: list[MemorySearchHit]
    total: int
    rewritten_query: str
    model_usage: list[MemorySearchModelUsage]
