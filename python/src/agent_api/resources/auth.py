from __future__ import annotations

import asyncio
import time
from collections.abc import Callable
from typing import Any

from agent_api._http import AsyncHTTPClient, SyncHTTPClient

DeviceAuthPollCallback = Callable[[dict[str, Any]], None]


class DeviceAuthFlowError(RuntimeError):
    """Raised when device/browser login expires, is consumed, or times out."""

    def __init__(self, message: str, result: dict[str, Any] | None = None) -> None:
        super().__init__(message)
        self.result = result


class AuthAPI:
    """Browser/device authentication helpers for CLI and desktop integrations."""

    def __init__(self, http: SyncHTTPClient) -> None:
        self._http = http

    def start_device_auth(self, *, client_name: str | None = None) -> dict[str, Any]:
        return self._http.request("POST", "/v1/auth/device/start", _drop_none({"client_name": client_name}))

    def poll_device_auth(self, *, device_code: str) -> dict[str, Any]:
        _require_device_code(device_code)
        return self._http.request("POST", "/v1/auth/device/poll", {"device_code": device_code})

    def wait_for_device_auth(
        self,
        *,
        device_code: str,
        interval_seconds: int | float | None = None,
        timeout: float | None = None,
        on_poll: DeviceAuthPollCallback | None = None,
    ) -> dict[str, Any]:
        _require_device_code(device_code)
        started = time.monotonic()
        while True:
            result = self.poll_device_auth(device_code=device_code)
            if on_poll is not None:
                on_poll(result)
            status = str(result.get("status") or "")
            if status == "approved" and result.get("access_token") and result.get("refresh_token"):
                return result
            if status in {"expired", "consumed"}:
                raise DeviceAuthFlowError(str(result.get("message") or f"Device auth {status}"), result)
            effective_timeout = _effective_timeout(timeout, started, result)
            if effective_timeout is not None and time.monotonic() - started >= effective_timeout:
                raise DeviceAuthFlowError("Device auth timed out", result)
            time.sleep(_poll_interval(interval_seconds, result))


class AsyncAuthAPI:
    """Async browser/device authentication helpers for CLI and desktop integrations."""

    def __init__(self, http: AsyncHTTPClient) -> None:
        self._http = http

    async def start_device_auth(self, *, client_name: str | None = None) -> dict[str, Any]:
        return await self._http.request("POST", "/v1/auth/device/start", _drop_none({"client_name": client_name}))

    async def poll_device_auth(self, *, device_code: str) -> dict[str, Any]:
        _require_device_code(device_code)
        return await self._http.request("POST", "/v1/auth/device/poll", {"device_code": device_code})

    async def wait_for_device_auth(
        self,
        *,
        device_code: str,
        interval_seconds: int | float | None = None,
        timeout: float | None = None,
        on_poll: DeviceAuthPollCallback | None = None,
    ) -> dict[str, Any]:
        _require_device_code(device_code)
        started = time.monotonic()
        while True:
            result = await self.poll_device_auth(device_code=device_code)
            if on_poll is not None:
                on_poll(result)
            status = str(result.get("status") or "")
            if status == "approved" and result.get("access_token") and result.get("refresh_token"):
                return result
            if status in {"expired", "consumed"}:
                raise DeviceAuthFlowError(str(result.get("message") or f"Device auth {status}"), result)
            effective_timeout = _effective_timeout(timeout, started, result)
            if effective_timeout is not None and time.monotonic() - started >= effective_timeout:
                raise DeviceAuthFlowError("Device auth timed out", result)
            await asyncio.sleep(_poll_interval(interval_seconds, result))


def _drop_none(value: dict[str, Any]) -> dict[str, Any]:
    return {key: item for key, item in value.items() if item is not None}


def _require_device_code(device_code: str) -> None:
    if not str(device_code or "").strip():
        raise ValueError("device_code is required")


def _poll_interval(interval_seconds: int | float | None, result: dict[str, Any]) -> float:
    raw = interval_seconds if interval_seconds is not None else result.get("interval_seconds", 5)
    try:
        return max(1.0, float(raw))
    except (TypeError, ValueError):
        return 5.0


def _effective_timeout(timeout: float | None, started: float, result: dict[str, Any]) -> float | None:
    if timeout is not None:
        return max(0.0, float(timeout))
    expires_at = result.get("expires_at")
    if not expires_at:
        return None
    return max(0.0, float(expires_at) - time.time() + (time.monotonic() - started))
