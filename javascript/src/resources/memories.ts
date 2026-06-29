import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type { MemorySearchParams, MemorySearchResponse } from "../types/memories.js";

export class MemoriesResource {
  constructor(private readonly http: HTTPClient) {}

  search(params: MemorySearchParams, options?: RequestOptions): Promise<MemorySearchResponse> {
    return this.http.request<MemorySearchResponse>("POST", "/v1/memories/search", params, options);
  }
}
