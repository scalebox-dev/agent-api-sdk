from __future__ import annotations

import os
from collections.abc import Mapping

import httpx

from agent_api._http import AsyncHTTPClient
from agent_api._version import DEFAULT_MAX_RETRIES, DEFAULT_STREAM_TIMEOUT, DEFAULT_TIMEOUT
from agent_api.resources.auth import AsyncAuthAPI
from agent_api.resources.catalog import AsyncModelsAPI, AsyncPresetsAPI, AsyncToolsAPI
from agent_api.resources.responses import AsyncResponsesAPI
from agent_api.resources.skills import AsyncSkillsAPI
from agent_api.resources.volumes import AsyncVolumesAPI


class AsyncAgentAPI:
    """Asynchronous production client for the Managed Agent API."""

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str | None = None,
        timeout: float | httpx.Timeout | None = None,
        stream_timeout: float | None = None,
        max_retries: int | None = None,
        default_headers: Mapping[str, str] | None = None,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        self.api_key = api_key or os.environ.get("AGENT_API_KEY")
        self.base_url = (base_url or os.environ.get("AGENT_API_BASE_URL") or "https://api.agentsway.dev").rstrip("/")
        self.timeout = float(timeout) if isinstance(timeout, (int, float)) else DEFAULT_TIMEOUT
        self.stream_timeout = stream_timeout if stream_timeout is not None else DEFAULT_STREAM_TIMEOUT
        self.max_retries = max_retries if max_retries is not None else DEFAULT_MAX_RETRIES
        self.default_headers = dict(default_headers or {})
        self._client = http_client or httpx.AsyncClient(timeout=timeout or DEFAULT_TIMEOUT)
        self._http = AsyncHTTPClient(
            base_url=self.base_url,
            api_key=self.api_key,
            timeout=self.timeout,
            stream_timeout=self.stream_timeout,
            max_retries=self.max_retries,
            default_headers=self.default_headers,
            http_client=self._client,
        )
        self.responses = AsyncResponsesAPI(self._http, "/v1/responses")
        self.agent = AsyncResponsesAPI(self._http, "/v1/agent")
        self.models = AsyncModelsAPI(self._http)
        self.presets = AsyncPresetsAPI(self._http)
        self.tools = AsyncToolsAPI(self._http)
        self.volumes = AsyncVolumesAPI(self._http)
        self.skills = AsyncSkillsAPI(self._http)
        self.auth = AsyncAuthAPI(self._http)

    async def start_device_auth(self, *, client_name: str | None = None) -> dict[str, object]:
        return await self.auth.start_device_auth(client_name=client_name)

    async def poll_device_auth(self, *, device_code: str) -> dict[str, object]:
        return await self.auth.poll_device_auth(device_code=device_code)

    async def wait_for_device_auth(
        self,
        *,
        device_code: str,
        interval_seconds: int | float | None = None,
        timeout: float | None = None,
    ) -> dict[str, object]:
        return await self.auth.wait_for_device_auth(
            device_code=device_code,
            interval_seconds=interval_seconds,
            timeout=timeout,
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def __aenter__(self) -> AsyncAgentAPI:
        return self

    async def __aexit__(self, *_exc: object) -> None:
        await self.close()
