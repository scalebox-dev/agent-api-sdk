export interface Annotation {
  type?: string;
  title?: string;
  url?: string;
  start_index?: number;
  end_index?: number;
}

export interface ContentPart {
  type: "input_text" | "output_text" | "input_image" | "input_document";
  text?: string;
  image_url?: string;
  mime_type?: string;
  filename?: string;
  document_url?: string;
  annotations?: Annotation[];
  logprobs?: unknown[];
}

export interface InputMessage {
  type: "message";
  role: "system" | "developer" | "user" | "assistant";
  content: string | ContentPart[];
}

export interface FunctionCallInput {
  type: "function_call";
  call_id: string;
  name: string;
  arguments: string;
  thought_signature?: string;
}

export interface FunctionCallOutputInput {
  type: "function_call_output";
  call_id: string;
  output: string;
  name?: string;
  thought_signature?: string;
}

export type InputItem = InputMessage | FunctionCallInput | FunctionCallOutputInput;
export type Input = string | InputItem[];

export interface ReasoningConfig {
  effort?: "none" | "minimal" | "low" | "medium" | "high" | "xhigh";
}

export interface ResponseFormat {
  type: "json_schema";
  json_schema?: {
    name: string;
    description?: string;
    schema: Record<string, unknown>;
    strict?: boolean;
  };
}

export interface MemoryOptions {
  enabled?: boolean;
  read?: boolean;
  write?: boolean;
}
