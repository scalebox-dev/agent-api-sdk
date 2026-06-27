from __future__ import annotations

import os
from collections.abc import Mapping

import httpx

from agent_api._http import SyncHTTPClient
from agent_api._version import DEFAULT_MAX_RETRIES, DEFAULT_STREAM_TIMEOUT, DEFAULT_TIMEOUT
from agent_api.resources.auth import AuthAPI
from agent_api.resources.catalog import ModelsAPI, PresetsAPI, ToolsAPI
from agent_api.resources.responses import ResponsesAPI
from agent_api.resources.safety_identifiers import SafetyIdentifiersAPI
from agent_api.resources.skills import SkillsAPI
from agent_api.resources.volumes import VolumesAPI


class AgentAPI:
    """Synchronous production client for the Managed Agent API."""

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str | None = None,
        timeout: float | httpx.Timeout | None = None,
        stream_timeout: float | None = None,
        max_retries: int | None = None,
        default_headers: Mapping[str, str] | None = None,
        http_client: httpx.Client | None = None,
    ) -> None:
        self.api_key = api_key or os.environ.get("AGENT_API_KEY")
        self.base_url = (base_url or os.environ.get("AGENT_API_BASE_URL") or "https://api.agentsway.dev").rstrip("/")
        self.timeout = float(timeout) if isinstance(timeout, (int, float)) else DEFAULT_TIMEOUT
        self.stream_timeout = stream_timeout if stream_timeout is not None else DEFAULT_STREAM_TIMEOUT
        self.max_retries = max_retries if max_retries is not None else DEFAULT_MAX_RETRIES
        self.default_headers = dict(default_headers or {})
        self._client = http_client or httpx.Client(timeout=timeout or DEFAULT_TIMEOUT)
        self._http = SyncHTTPClient(
            base_url=self.base_url,
            api_key=self.api_key,
            timeout=self.timeout,
            stream_timeout=self.stream_timeout,
            max_retries=self.max_retries,
            default_headers=self.default_headers,
            http_client=self._client,
        )
        self.responses = ResponsesAPI(self._http, "/v1/responses")
        self.agent = ResponsesAPI(self._http, "/v1/agent")
        self.models = ModelsAPI(self._http)
        self.presets = PresetsAPI(self._http)
        self.tools = ToolsAPI(self._http)
        self.volumes = VolumesAPI(self._http)
        self.skills = SkillsAPI(self._http)
        self.safety_identifiers = SafetyIdentifiersAPI(self._http)
        self.auth = AuthAPI(self._http)

    def start_device_auth(self, *, client_name: str | None = None) -> dict[str, object]:
        return self.auth.start_device_auth(client_name=client_name)

    def poll_device_auth(self, *, device_code: str) -> dict[str, object]:
        return self.auth.poll_device_auth(device_code=device_code)

    def refresh_browser_session(self, *, refresh_token: str) -> dict[str, object]:
        return self.auth.refresh_browser_session(refresh_token=refresh_token)

    def wait_for_device_auth(
        self,
        *,
        device_code: str,
        interval_seconds: int | float | None = None,
        timeout: float | None = None,
    ) -> dict[str, object]:
        return self.auth.wait_for_device_auth(
            device_code=device_code,
            interval_seconds=interval_seconds,
            timeout=timeout,
        )

    def close(self) -> None:
        self._client.close()

    def __enter__(self) -> AgentAPI:
        return self

    def __exit__(self, *_exc: object) -> None:
        self.close()
