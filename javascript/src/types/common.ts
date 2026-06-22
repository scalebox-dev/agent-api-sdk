export type JSONPrimitive = string | number | boolean | null;
export type JSONValue = JSONPrimitive | JSONValue[] | { [key: string]: JSONValue };

export type AgentCapabilityPreference = "off" | "auto" | "preferred" | "required";

export type ModelRoutingMode = "auto" | "chain";

export type ModelRoutingStrategy = "balanced" | "high-quality" | "cost-effective";

export interface ClientOptions {
  apiKey?: string;
  baseURL?: string;
  /** Default request timeout in milliseconds (non-streaming). */
  timeout?: number;
  /** Timeout for streaming agent runs in milliseconds. */
  streamTimeout?: number;
  /** Maximum number of automatic retries for retryable failures. Default 2. */
  maxRetries?: number;
  fetch?: typeof fetch;
  defaultHeaders?: Record<string, string>;
}

export interface RequestOptions {
  headers?: Record<string, string>;
  timeout?: number;
  /** Abort the underlying HTTP request or stream from caller-controlled UI/runtime state. */
  signal?: AbortSignal;
  /** Override automatic retries for this request. */
  maxRetries?: number;
}

export interface RetryConfig {
  maxRetries: number;
}

export interface ErrorInfo {
  message: string;
  type?: string;
  code?: string;
}

export interface Usage {
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  input_tokens_details?: Record<string, number>;
  output_tokens_details?: Record<string, number>;
  tool_calls_details?: Record<string, { invocation?: number }>;
  cost?: {
    currency?: string;
    input_cost?: number;
    output_cost?: number;
    tool_calls_cost?: number;
    cache_read_cost?: number;
    cache_creation_cost?: number;
    total_cost?: number;
  };
}

export type ResponseStatus = "completed" | "failed" | "in_progress" | "cancelled" | "requires_action";
export type OutputItemStatus = "completed" | "failed" | "in_progress";
