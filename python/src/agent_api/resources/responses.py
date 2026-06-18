from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from typing import Any, Literal, overload

from agent_api._http import SyncHTTPClient
from agent_api._utils import add_output_text, build_query, drop_none
from agent_api.pagination import AsyncPage, Page, PageResult
from agent_api.tool_validation import validate_unique_tool_names
from agent_api.types import AgentResponse, Input, ResponseListItem, ResponseStreamEvent
from agent_api.types.volumes import Volume


class ResponsesAPI:
    def __init__(self, http: SyncHTTPClient, path: str = "/v1/responses") -> None:
        self._http = http
        self._path = path

    @overload
    def create(self, *, input: Input, stream: Literal[True], **params: Any) -> Iterator[ResponseStreamEvent]: ...

    @overload
    def create(self, *, input: Input, stream: Literal[False] = False, **params: Any) -> AgentResponse: ...

    def create(self, *, input: Input, stream: bool = False, **params: Any) -> AgentResponse | Iterator[ResponseStreamEvent]:
        body = drop_none({"input": input, "stream": stream, **params})
        validate_unique_tool_names(body.get("tools"))  # type: ignore[arg-type]
        if stream:
            return self._http.stream("POST", self._path, body)
        response = self._http.request("POST", self._path, body)
        return add_output_text(response)

    def list(self, *, limit: int | None = None, page_token: str | None = None) -> dict[str, Any]:
        return self._http.request("GET", self._path + build_query({"limit": limit, "page_token": page_token}), None)

    def list_page(self, *, limit: int | None = None, page_token: str | None = None) -> Page[ResponseListItem, dict[str, Any]]:
        params = {"limit": limit, "page_token": page_token}

        def fetch(page_params: dict[str, Any]) -> PageResult[ResponseListItem]:
            payload = self.list(limit=page_params.get("limit"), page_token=page_params.get("page_token"))
            return PageResult(
                data=payload.get("data", []),
                has_more=bool(payload.get("has_more")),
                next_page_token=payload.get("next_page_token"),
            )

        return Page(fetch, params, fetch(params))

    def list_iterator(self, *, limit: int | None = None, page_token: str | None = None) -> Iterator[ResponseListItem]:
        return self.list_page(limit=limit, page_token=page_token).iter_all()

    def retrieve(self, response_id: str) -> AgentResponse:
        response = self._http.request("GET", f"{self._path}/{response_id}", None)
        return add_output_text(response)

    def cancel(self, response_id: str) -> dict[str, bool]:
        return self._http.request("POST", f"{self._path}/{response_id}/cancel", None)

    def list_children(self, response_id: str) -> dict[str, Any]:
        return self._http.request("GET", f"{self._path}/{response_id}/children", None)

    def list_events(
        self,
        response_id: str,
        *,
        after_sequence: int | None = None,
        view: Literal["timeline", "full"] | None = None,
    ) -> dict[str, Any]:
        return self._http.request(
            "GET",
            f"{self._path}/{response_id}/events" + build_query({"after_sequence": after_sequence, "view": view}),
            None,
        )

    def retrieve_volume(self, response_id: str) -> Volume:
        return self._http.request("GET", f"{self._path}/{response_id}/volume", None)


class AsyncResponsesAPI:
    def __init__(self, http: Any, path: str = "/v1/responses") -> None:
        self._http = http
        self._path = path

    @overload
    async def create(self, *, input: Input, stream: Literal[True], **params: Any) -> AsyncIterator[ResponseStreamEvent]: ...

    @overload
    async def create(self, *, input: Input, stream: Literal[False] = False, **params: Any) -> AgentResponse: ...

    async def create(
        self, *, input: Input, stream: bool = False, **params: Any
    ) -> AgentResponse | AsyncIterator[ResponseStreamEvent]:
        body = drop_none({"input": input, "stream": stream, **params})
        validate_unique_tool_names(body.get("tools"))  # type: ignore[arg-type]
        if stream:
            return self._http.stream("POST", self._path, body)
        response = await self._http.request("POST", self._path, body)
        return add_output_text(response)

    async def list(self, *, limit: int | None = None, page_token: str | None = None) -> dict[str, Any]:
        return await self._http.request("GET", self._path + build_query({"limit": limit, "page_token": page_token}), None)

    async def list_page(
        self, *, limit: int | None = None, page_token: str | None = None
    ) -> AsyncPage[ResponseListItem, dict[str, Any]]:
        params = {"limit": limit, "page_token": page_token}

        async def fetch(page_params: dict[str, Any]) -> PageResult[ResponseListItem]:
            payload = await self.list(limit=page_params.get("limit"), page_token=page_params.get("page_token"))
            return PageResult(
                data=payload.get("data", []),
                has_more=bool(payload.get("has_more")),
                next_page_token=payload.get("next_page_token"),
            )

        return AsyncPage(fetch, params, await fetch(params))

    async def list_iterator(
        self, *, limit: int | None = None, page_token: str | None = None
    ) -> AsyncIterator[ResponseListItem]:
        page = await self.list_page(limit=limit, page_token=page_token)
        async for item in page.iter_all():
            yield item

    async def retrieve(self, response_id: str) -> AgentResponse:
        response = await self._http.request("GET", f"{self._path}/{response_id}", None)
        return add_output_text(response)

    async def cancel(self, response_id: str) -> dict[str, bool]:
        return await self._http.request("POST", f"{self._path}/{response_id}/cancel", None)

    async def list_children(self, response_id: str) -> dict[str, Any]:
        return await self._http.request("GET", f"{self._path}/{response_id}/children", None)

    async def list_events(
        self,
        response_id: str,
        *,
        after_sequence: int | None = None,
        view: Literal["timeline", "full"] | None = None,
    ) -> dict[str, Any]:
        return await self._http.request(
            "GET",
            f"{self._path}/{response_id}/events" + build_query({"after_sequence": after_sequence, "view": view}),
            None,
        )

    async def retrieve_volume(self, response_id: str) -> Volume:
        return await self._http.request("GET", f"{self._path}/{response_id}/volume", None)
