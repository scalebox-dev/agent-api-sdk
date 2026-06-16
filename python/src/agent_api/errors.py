from __future__ import annotations

from typing import Any

import httpx


class APIError(Exception):
    """Base SDK error."""

    status_code: int | None
    body: Any
    code: str | None
    error_type: str | None
    request_id: str | None

    def __init__(
        self,
        message: str,
        *,
        status_code: int | None = None,
        body: Any = None,
        code: str | None = None,
        error_type: str | None = None,
        request_id: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.body = body
        self.code = code
        self.error_type = error_type
        self.request_id = request_id


class APIConnectionError(APIError):
    """Raised when the SDK cannot reach the API."""


class APIStatusError(APIError):
    """Raised for non-2xx API responses."""

    def __init__(
        self,
        message: str,
        *,
        status_code: int,
        response: httpx.Response,
        body: Any,
        code: str | None = None,
        error_type: str | None = None,
        request_id: str | None = None,
    ) -> None:
        super().__init__(
            message,
            status_code=status_code,
            body=body,
            code=code,
            error_type=error_type,
            request_id=request_id,
        )
        self.response = response


class AuthenticationError(APIStatusError):
    """Raised for 401 responses."""


class PermissionDeniedError(APIStatusError):
    """Raised for 403 responses."""


class NotFoundError(APIStatusError):
    """Raised for 404 responses."""


class BadRequestError(APIStatusError):
    """Raised for 400 responses."""


class RateLimitError(APIStatusError):
    """Raised for 429 responses."""

    retry_after_seconds: float | None

    def __init__(
        self,
        message: str,
        *,
        status_code: int,
        response: httpx.Response,
        body: Any,
        retry_after_seconds: float | None = None,
        code: str | None = None,
        error_type: str | None = None,
        request_id: str | None = None,
    ) -> None:
        super().__init__(
            message,
            status_code=status_code,
            response=response,
            body=body,
            code=code,
            error_type=error_type,
            request_id=request_id,
        )
        self.retry_after_seconds = retry_after_seconds


class InternalServerError(APIStatusError):
    """Raised for 5xx responses."""


def request_id_from_headers(response: httpx.Response) -> str | None:
    return response.headers.get("x-request-id") or response.headers.get("request-id")


def retry_after_seconds(response: httpx.Response) -> float | None:
    raw = response.headers.get("retry-after")
    if not raw:
        return None
    try:
        return max(0.0, float(raw))
    except ValueError:
        return None


def error_fields(body: Any) -> tuple[str, str | None, str | None]:
    if isinstance(body, dict):
        err = body.get("error")
        if isinstance(err, dict) and isinstance(err.get("message"), str):
            return err["message"], err.get("code"), err.get("type")
    return "", None, None


def parse_response_error(response: httpx.Response, body: Any) -> APIStatusError:
    message, code, error_type = error_fields(body)
    request_id = request_id_from_headers(response)
    final_message = message or f"API request failed with status {response.status_code}"
    common = {
        "status_code": response.status_code,
        "response": response,
        "body": body,
        "code": code,
        "error_type": error_type,
        "request_id": request_id,
    }
    if response.status_code == 400:
        return BadRequestError(final_message, **common)
    if response.status_code == 401:
        return AuthenticationError(final_message, **common)
    if response.status_code == 403:
        return PermissionDeniedError(final_message, **common)
    if response.status_code == 404:
        return NotFoundError(final_message, **common)
    if response.status_code == 429:
        return RateLimitError(
            final_message,
            retry_after_seconds=retry_after_seconds(response),
            **common,
        )
    if response.status_code >= 500:
        return InternalServerError(final_message, **common)
    return APIStatusError(final_message, **common)


def is_retryable_status(status_code: int) -> bool:
    return status_code in {408, 409, 429, 500, 502, 503, 504}
