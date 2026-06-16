import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type { ListPresetsResponse } from "../types/catalog.js";

export class PresetsResource {
  constructor(private readonly http: HTTPClient) {}

  list(options?: RequestOptions): Promise<ListPresetsResponse> {
    return this.http.request<ListPresetsResponse>("GET", "/v1/presets", undefined, options);
  }
}
