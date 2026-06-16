from __future__ import annotations

from typing import Any, Literal, TypedDict

class WebSearchTool(TypedDict, total=False):
    name: Literal["web_search"]
    type: Literal["search"]
    max_tokens: int
    max_tokens_per_page: int


class SmartWebSearchTool(TypedDict, total=False):
    name: Literal["smart_web_search"]
    type: Literal["search"]
    max_tokens: int
    max_tokens_per_page: int


class LiteWebSearchTool(TypedDict, total=False):
    name: Literal["lite_web_search"]
    type: Literal["search"]
    max_tokens: int
    max_tokens_per_page: int


class FetchURLTool(TypedDict, total=False):
    name: Literal["fetch_url"]
    type: Literal["url_reader"]


class FunctionTool(TypedDict, total=False):
    type: Literal["function"]
    name: str
    description: str
    parameters: dict[str, Any]
    strict: bool


class SkillTool(TypedDict, total=False):
    type: Literal["skill"]
    name: str
    version: str
    arguments: dict[str, Any]


Tool = WebSearchTool | SmartWebSearchTool | LiteWebSearchTool | FetchURLTool | FunctionTool | SkillTool
