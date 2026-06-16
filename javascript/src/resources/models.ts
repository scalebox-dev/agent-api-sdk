import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type { ListModelsResponse } from "../types/catalog.js";

export class ModelsResource {
  constructor(private readonly http: HTTPClient) {}

  list(options?: RequestOptions): Promise<ListModelsResponse> {
    return this.http.request<ListModelsResponse>("GET", "/v1/models", undefined, options);
  }
}
