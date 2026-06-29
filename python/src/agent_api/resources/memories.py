from __future__ import annotations

from typing import Any

from agent_api._utils import drop_none
from agent_api.types.memories import MemorySearchResponse


class MemoriesAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def search(
        self,
        *,
        query: str,
        limit: int | None = None,
        previous_response_id: str | None = None,
        tenant_search: bool | None = None,
        lang: str | None = None,
        semantic_weight: float | None = None,
    ) -> MemorySearchResponse:
        return self._http.request(
            "POST",
            "/v1/memories/search",
            drop_none(
                {
                    "query": query,
                    "limit": limit,
                    "previous_response_id": previous_response_id,
                    "tenant_search": tenant_search,
                    "lang": lang,
                    "semantic_weight": semantic_weight,
                }
            ),
        )


class AsyncMemoriesAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def search(
        self,
        *,
        query: str,
        limit: int | None = None,
        previous_response_id: str | None = None,
        tenant_search: bool | None = None,
        lang: str | None = None,
        semantic_weight: float | None = None,
    ) -> MemorySearchResponse:
        return await self._http.request(
            "POST",
            "/v1/memories/search",
            drop_none(
                {
                    "query": query,
                    "limit": limit,
                    "previous_response_id": previous_response_id,
                    "tenant_search": tenant_search,
                    "lang": lang,
                    "semantic_weight": semantic_weight,
                }
            ),
        )
