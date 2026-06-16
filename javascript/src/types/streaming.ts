import type { ErrorInfo, Usage } from "./common.js";
import type { AgentResponse, OutputItem, SearchResult, ToolInvocationResult, URLContent } from "./responses.js";

export interface ResponseStepEvent {
  step_id?: string;
  sequence_number?: number;
  step_type?: string;
  step_status?: string;
}

export interface ResponseModelCallEvent {
  step_id?: string;
  step_index?: number;
  step_type?: string;
  attempt?: number;
  status?: string;
  provider?: string;
  model?: string;
  model_chain?: string[];
  attempt_count?: number;
}

export type ResponseStreamEventType =
  | "response.created"
  | "response.in_progress"
  | "response.completed"
  | "response.failed"
  | "response.requires_action"
  | "response.plan.updated"
  | "response.output_item.added"
  | "response.output_item.done"
  | "response.output_text.delta"
  | "response.output_text.done"
  | "response.reasoning.started"
  | "response.reasoning.search_queries"
  | "response.reasoning.search_results"
  | "response.reasoning.fetch_url_queries"
  | "response.reasoning.fetch_url_results"
  | "response.reasoning.stopped"
  | "response.tool.invocation.completed"
  | "response.step.completed"
  | "response.step.failed"
  | "response.step.skipped"
  | "response.model.requested"
  | "response.model.completed"
  | "response.model.failed";

export interface ResponseStreamEvent {
  type: ResponseStreamEventType;
  sequence_number: number;
  response?: AgentResponse;
  item?: OutputItem;
  output_index?: number;
  item_id?: string;
  content_index?: number;
  delta?: string;
  text?: string;
  queries?: string[];
  urls?: string[];
  results?: SearchResult[];
  contents?: URLContent[];
  thought?: string;
  error?: ErrorInfo | null;
  usage?: Usage;
  step?: ResponseStepEvent;
  model_call?: ResponseModelCallEvent;
  tool_result?: ToolInvocationResult | null;
}
