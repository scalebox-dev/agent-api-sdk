import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type { ListToolsResponse } from "../types/tools.js";

export class ToolsResource {
  constructor(private readonly http: HTTPClient) {}

  list(options?: RequestOptions): Promise<ListToolsResponse> {
    return this.http.request<ListToolsResponse>("GET", "/v1/tools", undefined, options);
  }
}
