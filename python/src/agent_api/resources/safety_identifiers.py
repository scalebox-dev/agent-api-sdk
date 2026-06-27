from __future__ import annotations

from typing import Any

from agent_api._utils import build_query
from agent_api.types.safety_identifiers import ListSafetyIdentifierPartitionsResponse, SafetyIdentifierPartition


class SafetyIdentifiersAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    def list(self, *, page_size: int | None = None, page_token: str | None = None) -> ListSafetyIdentifierPartitionsResponse:
        return self._http.request(
            "GET",
            "/v1/safety_identifiers" + build_query({"page_size": page_size, "page_token": page_token}),
            None,
        )

    def lookup(self, safety_identifier: str) -> SafetyIdentifierPartition:
        return self._http.request("GET", "/v1/safety_identifiers/lookup" + build_query({"safety_identifier": safety_identifier}), None)


class AsyncSafetyIdentifiersAPI:
    def __init__(self, http: Any) -> None:
        self._http = http

    async def list(self, *, page_size: int | None = None, page_token: str | None = None) -> ListSafetyIdentifierPartitionsResponse:
        return await self._http.request(
            "GET",
            "/v1/safety_identifiers" + build_query({"page_size": page_size, "page_token": page_token}),
            None,
        )

    async def lookup(self, safety_identifier: str) -> SafetyIdentifierPartition:
        return await self._http.request("GET", "/v1/safety_identifiers/lookup" + build_query({"safety_identifier": safety_identifier}), None)
