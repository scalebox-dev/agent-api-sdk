from __future__ import annotations

from typing import Any, Literal, TypedDict

from agent_api.types.common import AgentCapabilityPreference, ErrorInfo, ResponseStatus, Usage
from agent_api.types.tools import Tool


class ToolInvocationResult(TypedDict, total=False):
    id: str
    tool_call_id: str
    tool_name: str
    status: str
    error: ErrorInfo | None
    metadata: dict[str, Any]
    response_summary: str
    response_summary_mime_type: str


class AgentResponse(TypedDict, total=False):
    id: str
    object: Literal["response"]
    created_at: int
    completed_at: int | None
    status: ResponseStatus
    model: str
    output: list[dict[str, Any]]
    output_text: str
    usage: Usage
    error: ErrorInfo | None
    metadata: dict[str, Any]
    instructions: str | None
    tools: list[Tool]
    tool_choice: Any
    parallel_tool_calls: bool
    previous_response_id: str | None
    parent_response_id: str | None
    root_response_id: str | None
    prompt_cache_key: str | None
    store: bool
    background: bool
    tool_results: list[ToolInvocationResult]
    plan: Any


class ResponseListItem(TypedDict, total=False):
    id: str
    status: ResponseStatus
    created_at: int
    completed_at: int | None
    model: str
    preset: str
    input_preview: str
    root_response_id: str
    background: bool


class ResponseChildItem(TypedDict, total=False):
    id: str
    status: ResponseStatus
    created_at: int
    completed_at: int | None
    root_response_id: str
    model: str


class CancelResponse(TypedDict):
    interrupted: bool


class ModelCapabilities(TypedDict, total=False):
    provider: str
    supports_streaming: bool
    supports_tools: bool
    supports_json_schema: bool
    supports_reasoning: bool
    context_window: int
    pricing: dict[str, Any]
    metadata: dict[str, Any]


class Model(TypedDict, total=False):
    id: str
    object: Literal["model"]
    owned_by: str
    capabilities: ModelCapabilities


class PresetPolicy(TypedDict, total=False):
    plan_mode_preference: AgentCapabilityPreference
    sub_agent_preference: AgentCapabilityPreference
    allowed_tools: list[str]
    max_steps: int


class Preset(TypedDict, total=False):
    preset: str
    prompt_version: str
    preset_metadata: dict[str, Any]
    policy: PresetPolicy
    max_output_tokens: int
    default_model: str
    model_chain: list[str]


class PublicTool(TypedDict, total=False):
    object: Literal["tool"]
    name: str
    type: str
    description: str
    parameters: dict[str, Any]
    max_tokens: int
    max_tokens_per_page: int
    version: str
