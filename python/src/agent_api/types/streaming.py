from __future__ import annotations

from typing import Any, Literal, TypedDict

from agent_api.types.common import ErrorInfo, Usage
from agent_api.types.responses import ToolInvocationResult


class ResponseStepEvent(TypedDict, total=False):
    step_id: str
    sequence_number: int
    step_type: str
    step_status: str


class ResponseModelCallEvent(TypedDict, total=False):
    step_id: str
    step_index: int
    step_type: str
    attempt: int
    status: str
    provider: str
    model: str
    model_chain: list[str]
    attempt_count: int


class ResponseStreamEvent(TypedDict, total=False):
    type: Literal[
        "response.created",
        "response.in_progress",
        "response.completed",
        "response.failed",
        "response.requires_action",
        "response.plan.updated",
        "response.output_item.added",
        "response.output_item.done",
        "response.output_text.delta",
        "response.output_text.done",
        "response.reasoning.started",
        "response.reasoning.search_queries",
        "response.reasoning.search_results",
        "response.reasoning.fetch_url_queries",
        "response.reasoning.fetch_url_results",
        "response.reasoning.stopped",
        "response.tool.invocation.completed",
        "response.step.completed",
        "response.step.failed",
        "response.step.skipped",
        "response.model.requested",
        "response.model.completed",
        "response.model.failed",
    ]
    sequence_number: int
    response: dict[str, Any]
    item: dict[str, Any]
    output_index: int
    item_id: str
    content_index: int
    delta: str
    text: str
    queries: list[str]
    urls: list[str]
    results: list[dict[str, Any]]
    contents: list[dict[str, Any]]
    thought: str
    error: ErrorInfo | None
    usage: Usage
    step: ResponseStepEvent
    model_call: ResponseModelCallEvent
    tool_result: ToolInvocationResult | None
