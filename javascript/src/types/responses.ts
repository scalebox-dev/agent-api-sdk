import type { AgentCapabilityPreference, ErrorInfo, ModelRoutingMode, ModelRoutingStrategy, OutputItemStatus, ResponseStatus, Usage } from "./common.js";
import type { ContentPart, Input, MemoryOptions, ReasoningConfig, ResponseFormat } from "./input.js";
import type { LocalSkillDescriptor, SkillReference } from "./skills.js";
import type { Tool } from "./tools.js";

export interface CallerContext {
  timezone?: string;
  locale?: string;
  locality?: string;
  extra?: unknown;
}

export interface SkillToolOptions {
  enabled?: boolean;
}

export interface ResponseCreateParamsBase {
  input: Input;
  instructions?: string;
  language_preference?: string;
  caller_context?: CallerContext;
  model?: string;
  models?: string[];
  model_routing?: ModelRoutingMode;
  routing_strategy?: ModelRoutingStrategy;
  preset?: "fast-search" | "pro-search" | "deep-research" | "advanced-deep-research" | string;
  max_output_tokens?: number;
  max_steps?: number;
  reasoning?: ReasoningConfig;
  response_format?: ResponseFormat;
  tools?: Tool[];
  tool_choice?: "auto" | "none" | "required" | Record<string, unknown>;
  parallel_tool_calls?: boolean;
  metadata?: Record<string, unknown>;
  store?: boolean;
  previous_response_id?: string;
  volume_id?: string;
  preferred_sites?: string[];
  skills?: SkillReference[];
  local_skills?: LocalSkillDescriptor[];
  skill_tool?: SkillToolOptions;
  prompt_cache_key?: string;
  memory?: MemoryOptions;
  plan_mode_preference?: AgentCapabilityPreference;
  sub_agent_preference?: AgentCapabilityPreference;
  safety_identifier?: string;
  user?: string;
  stream?: boolean;
}

export interface ResponseCreateParamsNonStreaming extends ResponseCreateParamsBase {
  stream?: false;
}

export interface ResponseCreateParamsStreaming extends ResponseCreateParamsBase {
  stream: true;
}

export type ResponseCreateParams = ResponseCreateParamsNonStreaming | ResponseCreateParamsStreaming;

export interface ToolInvocationResult {
  id?: string;
  tool_call_id?: string;
  tool_name?: string;
  status?: string;
  error?: ErrorInfo | null;
  metadata?: Record<string, unknown>;
  response_summary?: string;
  response_summary_mime_type?: string;
}

export interface MessageOutputItem {
  type: "message";
  id: string;
  status: OutputItemStatus;
  role: "assistant";
  content: ContentPart[];
}

export interface SearchResult {
  id: number;
  title: string;
  url: string;
  snippet: string;
  source?: string;
  date?: string;
  last_updated?: string;
}

export interface SearchResultsOutputItem {
  type: "search_results";
  queries?: string[];
  results: SearchResult[];
}

export interface URLContent {
  url: string;
  title: string;
  snippet: string;
}

export interface FetchURLResultsOutputItem {
  type: "fetch_url_results";
  contents: URLContent[];
}

export interface FunctionCallOutputItem {
  type: "function_call";
  id: string;
  status: OutputItemStatus;
  name: string;
  call_id: string;
  arguments: string;
  thought_signature?: string;
}

export type OutputItem =
  | MessageOutputItem
  | SearchResultsOutputItem
  | FetchURLResultsOutputItem
  | FunctionCallOutputItem;

export interface AgentResponse {
  id: string;
  object: "response";
  created_at: number;
  completed_at?: number | null;
  status: ResponseStatus;
  model: string;
  output: OutputItem[];
  output_text?: string;
  usage?: Usage;
  error?: ErrorInfo | null;
  metadata?: Record<string, unknown>;
  instructions?: string | null;
  tools?: Tool[];
  tool_choice?: unknown;
  parallel_tool_calls?: boolean;
  previous_response_id?: string | null;
  parent_response_id?: string | null;
  root_response_id?: string | null;
  prompt_cache_key?: string | null;
  store?: boolean;
  background?: boolean;
  tool_results?: ToolInvocationResult[];
  plan?: unknown;
  safety_identifier?: string;
}

export interface ResponseListItem {
  id: string;
  status: ResponseStatus;
  created_at: number;
  completed_at?: number | null;
  model?: string;
  preset?: string;
  input_preview?: string;
  root_response_id?: string;
  background?: boolean;
  safety_identifier?: string;
}

export interface ListResponsesParams {
  limit?: number;
  page_token?: string;
  safety_identifier?: string;
  user_id?: string;
}

export interface ListResponsesResponse {
  object: "list";
  data: ResponseListItem[];
  has_more: boolean;
  next_page_token?: string;
}

export interface ResponseChildItem {
  id: string;
  status: ResponseStatus;
  created_at: number;
  completed_at?: number | null;
  root_response_id?: string;
  model?: string;
}

export interface ListChildrenResponse {
  object: "list";
  data: ResponseChildItem[];
}

export interface ListEventsParams {
  after_sequence?: number;
  view?: "timeline" | "full";
}

export interface ListEventsResponse {
  data: import("./streaming.js").ResponseStreamEvent[];
}

export interface CancelResponse {
  interrupted: boolean;
}
