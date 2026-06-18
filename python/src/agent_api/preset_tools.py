from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass
from typing import Any, Literal, Protocol, cast

from agent_api.tool_validation import validate_unique_tool_names
from agent_api.types.responses import Preset, PublicTool
from agent_api.types.tools import Tool

UnknownPresetToolBehavior = Literal["stub", "omit", "error"]


class _ListResource(Protocol):
    def list(self) -> dict[str, Any]: ...


class _AsyncListResource(Protocol):
    async def list(self) -> dict[str, Any]: ...


class PresetToolCatalogClient(Protocol):
    presets: _ListResource
    tools: _ListResource


class AsyncPresetToolCatalogClient(Protocol):
    presets: _AsyncListResource
    tools: _AsyncListResource


@dataclass(frozen=True)
class ResolvePresetToolsResult:
    preset: Preset
    tools: list[Tool]
    missing_tool_names: list[str]


def resolve_preset_tools(
    client: PresetToolCatalogClient,
    *,
    preset: str,
    tools: Sequence[Tool] | None = None,
    presets: Sequence[Preset] | None = None,
    tool_catalog: Sequence[PublicTool] | None = None,
    unknown_preset_tool: UnknownPresetToolBehavior = "stub",
) -> ResolvePresetToolsResult:
    """Resolve preset allowed tools and append caller/client tools.

    Use this for hybrid apps that need to add local function tools while preserving
    the server-side tools provided by a preset. The returned list is intended for
    the normal CreateResponseRequest.tools field.
    """

    resolved_presets = presets if presets is not None else cast(list[Preset], client.presets.list().get("data", []))
    resolved_tools = tool_catalog if tool_catalog is not None else cast(list[PublicTool], client.tools.list().get("data", []))
    return resolve_preset_tools_from_catalog(
        preset=preset,
        tools=tools,
        presets=resolved_presets,
        tool_catalog=resolved_tools,
        unknown_preset_tool=unknown_preset_tool,
    )


async def async_resolve_preset_tools(
    client: AsyncPresetToolCatalogClient,
    *,
    preset: str,
    tools: Sequence[Tool] | None = None,
    presets: Sequence[Preset] | None = None,
    tool_catalog: Sequence[PublicTool] | None = None,
    unknown_preset_tool: UnknownPresetToolBehavior = "stub",
) -> ResolvePresetToolsResult:
    resolved_presets = presets if presets is not None else cast(list[Preset], (await client.presets.list()).get("data", []))
    resolved_tools = tool_catalog if tool_catalog is not None else cast(list[PublicTool], (await client.tools.list()).get("data", []))
    return resolve_preset_tools_from_catalog(
        preset=preset,
        tools=tools,
        presets=resolved_presets,
        tool_catalog=resolved_tools,
        unknown_preset_tool=unknown_preset_tool,
    )


def resolve_preset_tools_from_catalog(
    *,
    preset: str,
    tools: Sequence[Tool] | None = None,
    presets: Sequence[Preset] | None = None,
    tool_catalog: Sequence[PublicTool] | None = None,
    unknown_preset_tool: UnknownPresetToolBehavior = "stub",
) -> ResolvePresetToolsResult:
    preset_id = preset.strip()
    if not preset_id:
        raise ValueError("preset is required")

    matched = next((row for row in presets or [] if row.get("preset") == preset_id), None)
    if matched is None:
        raise ValueError(f"preset not found: {preset_id}")

    catalog_by_name = {name: tool for tool in tool_catalog or [] if (name := str(tool.get("name", "")).strip())}
    missing_tool_names: list[str] = []
    preset_tools: list[Tool] = []

    for name in matched.get("policy", {}).get("allowed_tools", []):
        trimmed = str(name).strip()
        if not trimmed:
            continue
        catalog_tool = catalog_by_name.get(trimmed)
        if catalog_tool is not None:
            preset_tools.append(public_tool_to_request_tool(catalog_tool))
            continue
        missing_tool_names.append(trimmed)
        if unknown_preset_tool == "error":
            raise ValueError(f"preset tool not found in catalog: {trimmed}")
        if unknown_preset_tool == "stub":
            preset_tools.append(cast(Tool, {"name": trimmed}))

    return ResolvePresetToolsResult(
        preset=matched,
        tools=merge_tools(preset_tools, tools or []),
        missing_tool_names=missing_tool_names,
    )


def merge_tools(*groups: Sequence[Tool]) -> list[Tool]:
    out: list[Tool] = []
    for group in groups:
        for tool in group:
            name = str(tool.get("name", "")).strip()
            if not name:
                raise ValueError("tools[].name is required")
            normalized = dict(tool)
            normalized["name"] = name
            out.append(cast(Tool, normalized))
    validate_unique_tool_names(out)
    return out


def public_tool_to_request_tool(tool: PublicTool) -> Tool:
    out: dict[str, Any] = {"name": tool["name"]}
    for key in ("type", "description", "parameters", "max_tokens", "max_tokens_per_page", "version"):
        if key in tool:
            out[key] = tool[key]  # type: ignore[literal-required]
    return cast(Tool, out)
