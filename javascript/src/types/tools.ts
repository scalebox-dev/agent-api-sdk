export interface WebSearchTool {
  name: "web_search";
  type?: "search";
  max_tokens?: number;
  max_tokens_per_page?: number;
}

export interface SmartWebSearchTool {
  name: "smart_web_search";
  type?: "search";
  max_tokens?: number;
  max_tokens_per_page?: number;
}

export interface LiteWebSearchTool {
  name: "lite_web_search";
  type?: "search";
  max_tokens?: number;
  max_tokens_per_page?: number;
}

export interface FetchURLTool {
  name: "fetch_url";
  type?: "url_reader";
}

export interface FunctionTool {
  type: "function";
  name: string;
  description?: string;
  parameters?: Record<string, unknown>;
  strict?: boolean;
}

export interface SkillTool {
  type: "skill";
  name: string;
  version?: string;
  arguments?: Record<string, unknown>;
}

export type Tool = WebSearchTool | SmartWebSearchTool | LiteWebSearchTool | FetchURLTool | FunctionTool | SkillTool;

export interface PublicTool {
  object: "tool";
  name: string;
  type?: string;
  description?: string;
  parameters?: Record<string, unknown>;
  max_tokens?: number;
  max_tokens_per_page?: number;
  version?: string;
}

export interface ListToolsResponse {
  object: "list";
  data: PublicTool[];
}
