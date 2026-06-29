from __future__ import annotations

from typing import Any, Literal, TypedDict

from agent_api.types.common import AgentCapabilityPreference
from agent_api.types.skills import LocalSkillDescriptor, SkillReference


class Annotation(TypedDict, total=False):
    type: str
    title: str
    url: str
    start_index: int
    end_index: int


class ContentPart(TypedDict, total=False):
    type: Literal["input_text", "output_text", "input_image", "input_document"]
    text: str
    image_url: str
    mime_type: str
    filename: str
    document_url: str
    annotations: list[Annotation]
    logprobs: list[Any]


class InputMessage(TypedDict):
    type: Literal["message"]
    role: Literal["system", "developer", "user", "assistant"]
    content: str | list[ContentPart]


class FunctionCallInput(TypedDict, total=False):
    type: Literal["function_call"]
    call_id: str
    name: str
    arguments: str
    thought_signature: str


class FunctionCallOutputInput(TypedDict, total=False):
    type: Literal["function_call_output"]
    call_id: str
    output: str
    name: str
    thought_signature: str


InputItem = InputMessage | FunctionCallInput | FunctionCallOutputInput
Input = str | list[InputItem]


class ReasoningConfig(TypedDict, total=False):
    effort: Literal["none", "minimal", "low", "medium", "high", "xhigh"]


class ResponseFormat(TypedDict, total=False):
    type: Literal["json_schema"]
    json_schema: dict[str, Any]


class MemoryOptions(TypedDict, total=False):
    enabled: bool
    read: bool
    write: bool
    tenant_search: bool


class SkillToolOptions(TypedDict, total=False):
    enabled: bool
    tenant_search: bool


class ResponseCreateParams(TypedDict, total=False):
    input: Input
    instructions: str
    language_preference: str
    model: str
    models: list[str]
    preset: str
    max_output_tokens: int
    max_steps: int
    reasoning: ReasoningConfig
    response_format: ResponseFormat
    stream: bool
    tools: list[Any]
    tool_choice: str | dict[str, Any]
    parallel_tool_calls: bool
    metadata: dict[str, Any]
    user: str
    store: bool
    previous_response_id: str
    volume_id: str
    preferred_sites: list[str]
    skills: list[SkillReference]
    local_skills: list[LocalSkillDescriptor]
    skill_tool: SkillToolOptions
    prompt_cache_key: str
    memory: MemoryOptions
    plan_mode_preference: AgentCapabilityPreference
    sub_agent_preference: AgentCapabilityPreference
    safety_identifier: str
