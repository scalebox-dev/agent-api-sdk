import { buildQuery } from "../internal/query.js";
import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type {
  ListSafetyIdentifiersParams,
  ListSafetyIdentifiersResponse,
  SafetyIdentifier,
} from "../types/safety-identifiers.js";

export class SafetyIdentifiersResource {
  constructor(private readonly http: HTTPClient) {}

  list(params: ListSafetyIdentifiersParams = {}, options?: RequestOptions): Promise<ListSafetyIdentifiersResponse> {
    return this.http.request<ListSafetyIdentifiersResponse>(
      "GET",
      `/v1/safety_identifiers${buildQuery({ page_size: params.page_size, page_token: params.page_token })}`,
      undefined,
      options,
    );
  }

  lookup(safetyIdentifier: string, options?: RequestOptions): Promise<SafetyIdentifier> {
    return this.http.request<SafetyIdentifier>(
      "GET",
      `/v1/safety_identifiers/lookup${buildQuery({ safety_identifier: safetyIdentifier })}`,
      undefined,
      options,
    );
  }
}
