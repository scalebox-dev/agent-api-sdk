import type { Usage } from "./common.js";

export interface MemorySearchParams {
  query: string;
  limit?: number;
  previous_response_id?: string;
  tenant_search?: boolean;
  lang?: string;
  semantic_weight?: number;
}

export interface MemorySearchHit {
  id: string;
  score?: number;
  thread_id?: string;
  created_at?: number;
  fact: string;
  tenant_id?: string;
  user_id?: string;
  response_id?: string;
  metadata?: unknown;
  metadata_text?: string;
}

export interface MemorySearchModelUsage {
  source?: string;
  phase?: string;
  provider?: string;
  model?: string;
  attempt_index?: number;
  status?: string;
  usage?: Usage;
}

export interface MemorySearchResponse {
  object: "memory_search_result";
  data: MemorySearchHit[];
  total?: number;
  rewritten_query?: string;
  model_usage?: MemorySearchModelUsage[];
}
