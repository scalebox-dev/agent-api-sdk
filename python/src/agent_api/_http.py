from __future__ import annotations

import random
import time
from collections.abc import AsyncIterator, Awaitable, Callable, Iterator, Mapping
from inspect import isawaitable
from typing import Any

import httpx

from agent_api._utils import build_query
from agent_api._version import DEFAULT_MAX_RETRIES, DEFAULT_STREAM_TIMEOUT, DEFAULT_TIMEOUT, USER_AGENT
from agent_api.errors import (
    APIConnectionError,
    APIError,
    APIStatusError,
    RateLimitError,
    is_retryable_status,
    parse_response_error,
)
from agent_api.streaming import aiter_sse, iter_sse


class SyncHTTPClient:
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str | None,
        api_key_provider: Callable[[], str | None] | None,
        timeout: float,
        stream_timeout: float,
        max_retries: int,
        default_headers: Mapping[str, str],
        http_client: httpx.Client,
    ) -> None:
        self.base_url = base_url
        self.api_key = api_key
        self.api_key_provider = api_key_provider
        self.timeout = timeout
        self.stream_timeout = stream_timeout
        self.max_retries = max_retries
        self.default_headers = dict(default_headers)
        self._client = http_client

    def request(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> Any:
        response = self._request_response(
            method,
            path,
            body,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )
        return response.json()

    def request_binary(
        self,
        method: str,
        path: str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> tuple[bytes, httpx.Headers]:
        response = self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )
        return response.content, response.headers

    def request_raw(
        self,
        method: str,
        path: str,
        content: bytes | str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> Any:
        response = self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
            content=content,
        )
        return response.json()

    def request_void(
        self,
        method: str,
        path: str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> None:
        self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )

    def stream(
        self,
        method: str,
        path: str,
        body: dict[str, Any],
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> Iterator[Any]:
        effective_timeout = timeout if timeout is not None else self.stream_timeout
        retries = self.max_retries if max_retries is None else max_retries
        attempt = 0
        while True:
            try:
                headers = self._headers(stream=True, extra=extra_headers)
                with self._client.stream(
                    method,
                    self.base_url + path,
                    json=body,
                    headers=headers,
                    timeout=effective_timeout,
                ) as response:
                    if not response.is_success:
                        try:
                            payload: Any = response.json()
                        except ValueError:
                            payload = response.text
                        raise parse_response_error(response, payload)
                    yield from iter_sse(response.iter_lines())
                return
            except APIError as exc:
                if attempt >= retries:
                    raise
                retryable = isinstance(exc, APIConnectionError) or (
                    isinstance(exc, APIStatusError) and exc.status_code is not None and is_retryable_status(exc.status_code)
                )
                if not retryable:
                    raise
                attempt += 1
                time.sleep(_retry_delay_seconds(exc, attempt))
            except httpx.TimeoutException as exc:
                raise APIConnectionError(f"Request timed out after {effective_timeout}s") from exc
            except httpx.HTTPError as exc:
                raise APIConnectionError(str(exc)) from exc

    def _request_response(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        stream: bool,
        timeout: float | None,
        max_retries: int | None,
        extra_headers: Mapping[str, str] | None,
        content: bytes | str | None = None,
    ) -> httpx.Response:
        retries = self.max_retries if max_retries is None else max_retries
        attempt = 0
        while True:
            try:
                return self._request_once(
                    method,
                    path,
                    body,
                    stream=stream,
                    timeout=timeout,
                    extra_headers=extra_headers,
                    content=content,
                )
            except APIError as exc:
                if attempt >= retries:
                    raise
                retryable = isinstance(exc, APIConnectionError) or (
                    isinstance(exc, APIStatusError) and exc.status_code is not None and is_retryable_status(exc.status_code)
                )
                if not retryable:
                    raise
                attempt += 1
                time.sleep(_retry_delay_seconds(exc, attempt))

    def _request_once(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        stream: bool,
        timeout: float | None,
        extra_headers: Mapping[str, str] | None,
        content: bytes | str | None = None,
    ) -> httpx.Response:
        effective_timeout = timeout if timeout is not None else (self.stream_timeout if stream else self.timeout)
        headers = self._headers(stream=stream, extra=extra_headers)
        try:
            if stream:
                request = self._client.build_request(
                    method,
                    self.base_url + path,
                    json=body,
                    headers=headers,
                    timeout=effective_timeout,
                )
                response = self._client.send(request, stream=True)
            else:
                response = self._client.request(
                    method,
                    self.base_url + path,
                    json=body if content is None else None,
                    content=content,
                    headers=headers,
                    timeout=effective_timeout,
                )
        except httpx.TimeoutException as exc:
            raise APIConnectionError(f"Request timed out after {effective_timeout}s") from exc
        except httpx.HTTPError as exc:
            raise APIConnectionError(str(exc)) from exc

        if response.is_success:
            return response
        try:
            payload: Any = response.json()
        except ValueError:
            payload = response.text
        raise parse_response_error(response, payload)

    def _headers(self, *, stream: bool, extra: Mapping[str, str] | None) -> dict[str, str]:
        headers = {
            "Accept": "text/event-stream" if stream else "application/json",
            "User-Agent": USER_AGENT,
            **self.default_headers,
            **dict(extra or {}),
        }
        api_key = self._api_key()
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        return headers

    def _api_key(self) -> str | None:
        if self.api_key_provider is None:
            return self.api_key
        provided = self.api_key_provider()
        return provided or self.api_key


class AsyncHTTPClient:
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str | None,
        api_key_provider: Callable[[], str | Awaitable[str | None] | None] | None,
        timeout: float,
        stream_timeout: float,
        max_retries: int,
        default_headers: Mapping[str, str],
        http_client: httpx.AsyncClient,
    ) -> None:
        self.base_url = base_url
        self.api_key = api_key
        self.api_key_provider = api_key_provider
        self.timeout = timeout
        self.stream_timeout = stream_timeout
        self.max_retries = max_retries
        self.default_headers = dict(default_headers)
        self._client = http_client

    async def request(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> Any:
        response = await self._request_response(
            method,
            path,
            body,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )
        return response.json()

    async def request_binary(
        self,
        method: str,
        path: str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> tuple[bytes, httpx.Headers]:
        response = await self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )
        return response.content, response.headers

    async def request_raw(
        self,
        method: str,
        path: str,
        content: bytes | str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> Any:
        response = await self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
            content=content,
        )
        return response.json()

    async def request_void(
        self,
        method: str,
        path: str,
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> None:
        await self._request_response(
            method,
            path,
            None,
            stream=False,
            timeout=timeout,
            max_retries=max_retries,
            extra_headers=extra_headers,
        )

    async def stream(
        self,
        method: str,
        path: str,
        body: dict[str, Any],
        *,
        timeout: float | None = None,
        max_retries: int | None = None,
        extra_headers: Mapping[str, str] | None = None,
    ) -> AsyncIterator[Any]:
        effective_timeout = timeout if timeout is not None else self.stream_timeout
        retries = self.max_retries if max_retries is None else max_retries
        attempt = 0
        while True:
            try:
                headers = await self._headers(stream=True, extra=extra_headers)
                async with self._client.stream(
                    method,
                    self.base_url + path,
                    json=body,
                    headers=headers,
                    timeout=effective_timeout,
                ) as response:
                    if not response.is_success:
                        try:
                            payload: Any = response.json()
                        except ValueError:
                            payload = response.text
                        raise parse_response_error(response, payload)
                    async for event in aiter_sse(response.aiter_lines()):
                        yield event
                return
            except APIError as exc:
                if attempt >= retries:
                    raise
                retryable = isinstance(exc, APIConnectionError) or (
                    isinstance(exc, APIStatusError) and exc.status_code is not None and is_retryable_status(exc.status_code)
                )
                if not retryable:
                    raise
                attempt += 1
                await _async_sleep(_retry_delay_seconds(exc, attempt))
            except httpx.TimeoutException as exc:
                raise APIConnectionError(f"Request timed out after {effective_timeout}s") from exc
            except httpx.HTTPError as exc:
                raise APIConnectionError(str(exc)) from exc

    async def _request_response(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        stream: bool,
        timeout: float | None,
        max_retries: int | None,
        extra_headers: Mapping[str, str] | None,
        content: bytes | str | None = None,
    ) -> httpx.Response:
        retries = self.max_retries if max_retries is None else max_retries
        attempt = 0
        while True:
            try:
                return await self._request_once(
                    method,
                    path,
                    body,
                    stream=stream,
                    timeout=timeout,
                    extra_headers=extra_headers,
                    content=content,
                )
            except APIError as exc:
                if attempt >= retries:
                    raise
                retryable = isinstance(exc, APIConnectionError) or (
                    isinstance(exc, APIStatusError) and exc.status_code is not None and is_retryable_status(exc.status_code)
                )
                if not retryable:
                    raise
                attempt += 1
                await _async_sleep(_retry_delay_seconds(exc, attempt))

    async def _request_once(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None,
        *,
        stream: bool,
        timeout: float | None,
        extra_headers: Mapping[str, str] | None,
        content: bytes | str | None = None,
    ) -> httpx.Response:
        effective_timeout = timeout if timeout is not None else (self.stream_timeout if stream else self.timeout)
        headers = await self._headers(stream=stream, extra=extra_headers)
        try:
            if stream:
                request = self._client.build_request(
                    method,
                    self.base_url + path,
                    json=body,
                    headers=headers,
                    timeout=effective_timeout,
                )
                response = await self._client.send(request, stream=True)
            else:
                response = await self._client.request(
                    method,
                    self.base_url + path,
                    json=body if content is None else None,
                    content=content,
                    headers=headers,
                    timeout=effective_timeout,
                )
        except httpx.TimeoutException as exc:
            raise APIConnectionError(f"Request timed out after {effective_timeout}s") from exc
        except httpx.HTTPError as exc:
            raise APIConnectionError(str(exc)) from exc

        if response.is_success:
            return response
        try:
            payload: Any = response.json()
        except ValueError:
            payload = response.text
        raise parse_response_error(response, payload)

    async def _headers(self, *, stream: bool, extra: Mapping[str, str] | None) -> dict[str, str]:
        headers = {
            "Accept": "text/event-stream" if stream else "application/json",
            "User-Agent": USER_AGENT,
            **self.default_headers,
            **dict(extra or {}),
        }
        api_key = await self._api_key()
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        return headers

    async def _api_key(self) -> str | None:
        if self.api_key_provider is None:
            return self.api_key
        provided = self.api_key_provider()
        if isawaitable(provided):
            provided = await provided
        return provided or self.api_key


def _retry_delay_seconds(error: APIError, attempt: int) -> float:
    if isinstance(error, RateLimitError) and error.retry_after_seconds is not None:
        return error.retry_after_seconds
    base = 0.25
    cap = 8.0
    exponential = min(cap, base * (2 ** (attempt - 1)))
    jitter = random.uniform(0, 0.1)
    return exponential + jitter


async def _async_sleep(seconds: float) -> None:
    import asyncio

    await asyncio.sleep(seconds)


__all__ = ["SyncHTTPClient", "AsyncHTTPClient"]
