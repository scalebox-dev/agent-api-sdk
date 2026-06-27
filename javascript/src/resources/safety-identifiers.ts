import { buildQuery } from "../internal/query.js";
import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type {
  ListSafetyIdentifierPartitionsParams,
  ListSafetyIdentifierPartitionsResponse,
  SafetyIdentifierPartition,
} from "../types/safety-identifiers.js";

export class SafetyIdentifiersResource {
  constructor(private readonly http: HTTPClient) {}

  list(params: ListSafetyIdentifierPartitionsParams = {}, options?: RequestOptions): Promise<ListSafetyIdentifierPartitionsResponse> {
    return this.http.request<ListSafetyIdentifierPartitionsResponse>(
      "GET",
      `/v1/safety_identifiers${buildQuery({ page_size: params.page_size, page_token: params.page_token })}`,
      undefined,
      options,
    );
  }

  lookup(safetyIdentifier: string, options?: RequestOptions): Promise<SafetyIdentifierPartition> {
    return this.http.request<SafetyIdentifierPartition>(
      "GET",
      `/v1/safety_identifiers/lookup${buildQuery({ safety_identifier: safetyIdentifier })}`,
      undefined,
      options,
    );
  }
}
