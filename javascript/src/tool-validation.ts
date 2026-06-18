import type { Tool } from "./types/tools.js";

export function validateUniqueToolNames(tools: readonly Tool[] | undefined): void {
  if (!tools) return;
  const seen = new Set<string>();
  for (const tool of tools) {
    const name = tool.name?.trim();
    if (!name) {
      throw new Error("tools[].name is required");
    }
    if (seen.has(name)) {
      throw new Error(`duplicate tools[].name: ${name}`);
    }
    seen.add(name);
  }
}
