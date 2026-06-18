import type { ListPresetsResponse, Preset } from "./types/catalog.js";
import type { ListToolsResponse, PublicTool, Tool } from "./types/tools.js";
import { validateUniqueToolNames } from "./tool-validation.js";

export interface PresetToolCatalogClient {
  presets: {
    list(): Promise<ListPresetsResponse>;
  };
  tools: {
    list(): Promise<ListToolsResponse>;
  };
}

export type UnknownPresetToolBehavior = "stub" | "omit" | "error";

export interface ResolvePresetToolsOptions {
  /**
   * Preset id whose policy.allowed_tools should be resolved.
   */
  preset: string;
  /**
   * Additional caller/client tools to append after the preset tools.
   */
  tools?: readonly Tool[];
  /**
   * Optional pre-fetched preset catalog. Use this when your app caches discovery
   * responses and wants deterministic request construction.
   */
  presets?: readonly Preset[];
  /**
   * Optional pre-fetched tool catalog.
   */
  toolCatalog?: readonly PublicTool[];
  /**
   * Behavior when a preset allows a tool that is not present in the supplied
   * or fetched tool catalog. "stub" keeps a name-only tool definition so the
   * backend can still enrich it by name.
   */
  unknownPresetTool?: UnknownPresetToolBehavior;
}

export interface ResolvePresetToolsResult {
  preset: Preset;
  tools: Tool[];
  missingToolNames: string[];
}

/**
 * Resolve a preset's allowed tool names into concrete request tools and append
 * caller-provided tools. This is a convenience for hybrid apps that need to add
 * local/client tools while preserving the preset's default server tools.
 *
 * The Agent API request surface remains OpenAI-compatible: the returned array
 * is intended for the normal CreateResponseRequest.tools field.
 */
export async function resolvePresetTools(
  client: PresetToolCatalogClient,
  options: ResolvePresetToolsOptions,
): Promise<ResolvePresetToolsResult> {
  const presets = options.presets ?? (await client.presets.list()).data;
  const toolCatalog = options.toolCatalog ?? (await client.tools.list()).data;
  return resolvePresetToolsFromCatalog({ ...options, presets, toolCatalog });
}

export function resolvePresetToolsFromCatalog(options: ResolvePresetToolsOptions): ResolvePresetToolsResult {
  const presetId = options.preset.trim();
  if (!presetId) {
    throw new Error("preset is required");
  }
  const preset = options.presets?.find((row) => row.preset === presetId);
  if (!preset) {
    throw new Error(`preset not found: ${presetId}`);
  }

  const catalogByName = new Map<string, PublicTool>();
  for (const tool of options.toolCatalog ?? []) {
    const name = tool.name?.trim();
    if (name) catalogByName.set(name, tool);
  }

  const missingToolNames: string[] = [];
  const presetTools: Tool[] = [];
  const unknownPresetTool = options.unknownPresetTool ?? "stub";

  for (const name of preset.policy?.allowed_tools ?? []) {
    const trimmed = name.trim();
    if (!trimmed) continue;
    const catalogTool = catalogByName.get(trimmed);
    if (catalogTool) {
      presetTools.push(publicToolToRequestTool(catalogTool));
      continue;
    }
    missingToolNames.push(trimmed);
    if (unknownPresetTool === "error") {
      throw new Error(`preset tool not found in catalog: ${trimmed}`);
    }
    if (unknownPresetTool === "stub") {
      presetTools.push({ name: trimmed } as Tool);
    }
  }

  return {
    preset,
    tools: mergeTools(presetTools, options.tools ?? []),
    missingToolNames,
  };
}

export function mergeTools(...groups: Array<readonly Tool[]>): Tool[] {
  const out: Tool[] = [];
  for (const group of groups) {
    for (const tool of group) {
      const name = tool.name?.trim();
      if (!name) {
        throw new Error("tools[].name is required");
      }
      out.push({ ...tool, name } as Tool);
    }
  }
  validateUniqueToolNames(out);
  return out;
}

export function publicToolToRequestTool(tool: PublicTool): Tool {
  const out: Record<string, unknown> = { name: tool.name };
  if (tool.type) out.type = tool.type;
  if (tool.description) out.description = tool.description;
  if (tool.parameters) out.parameters = tool.parameters;
  if (tool.max_tokens != null) out.max_tokens = tool.max_tokens;
  if (tool.max_tokens_per_page != null) out.max_tokens_per_page = tool.max_tokens_per_page;
  if (tool.version) out.version = tool.version;
  return out as unknown as Tool;
}
