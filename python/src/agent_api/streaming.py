from __future__ import annotations

import json
from collections.abc import AsyncIterator, Iterator

import httpx

from agent_api.types import ResponseStreamEvent


def iter_sse(lines: Iterator[str]) -> Iterator[ResponseStreamEvent]:
    data: list[str] = []
    for line in lines:
        if line == "":
            if data:
                yield decode_sse(data)
                data = []
            continue
        if line.startswith("data:"):
            data.append(line[5:].lstrip())
    if data:
        yield decode_sse(data)


async def aiter_sse(lines: AsyncIterator[str]) -> AsyncIterator[ResponseStreamEvent]:
    data: list[str] = []
    async for line in lines:
        if line == "":
            if data:
                yield decode_sse(data)
                data = []
            continue
        if line.startswith("data:"):
            data.append(line[5:].lstrip())
    if data:
        yield decode_sse(data)


def decode_sse(data: list[str]) -> ResponseStreamEvent:
    payload = "\n".join(data)
    if payload == "[DONE]":
        return {"type": "response.completed", "sequence_number": -1}
    return json.loads(payload)
