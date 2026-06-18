import { addOutputText } from "../internal/output-text.js";
import { buildQuery } from "../internal/query.js";
import type { HTTPClient } from "../internal/http.js";
import { collectPage, Page } from "../pagination.js";
import { validateUniqueToolNames } from "../tool-validation.js";
import type { RequestOptions } from "../types/common.js";
import type {
  AgentResponse,
  CancelResponse,
  ListChildrenResponse,
  ListEventsParams,
  ListEventsResponse,
  ListResponsesParams,
  ListResponsesResponse,
  ResponseCreateParams,
  ResponseCreateParamsNonStreaming,
  ResponseCreateParamsStreaming,
  ResponseListItem,
} from "../types/responses.js";
import type { ResponseStreamEvent } from "../types/streaming.js";
import type { VolumeInfo } from "../types/volumes.js";

export class ResponsesResource {
  constructor(
    private readonly http: HTTPClient,
    private readonly path = "/v1/responses",
  ) {}

  create(params: ResponseCreateParamsNonStreaming, options?: RequestOptions): Promise<AgentResponse>;
  create(params: ResponseCreateParamsStreaming, options?: RequestOptions): Promise<AsyncIterable<ResponseStreamEvent>>;
  create(
    params: ResponseCreateParams,
    options?: RequestOptions,
  ): Promise<AgentResponse | AsyncIterable<ResponseStreamEvent>> {
    validateUniqueToolNames(params.tools);
    if (params.stream) {
      return this.http.stream<ResponseStreamEvent>(this.path, params, options);
    }
    return this.http.request<AgentResponse>("POST", this.path, params, options).then(addOutputText);
  }

  list(params: ListResponsesParams = {}, options?: RequestOptions): Promise<ListResponsesResponse> {
    return this.http.request<ListResponsesResponse>(
      "GET",
      `${this.path}${buildQuery({ limit: params.limit, page_token: params.page_token })}`,
      undefined,
      options,
    );
  }

  async listPage(params: ListResponsesParams = {}, options?: RequestOptions): Promise<Page<ResponseListItem, ListResponsesParams>> {
    return collectPage(
      (pageParams) => this.list(pageParams, options),
      params,
    );
  }

  listIterator(params: ListResponsesParams = {}, options?: RequestOptions): AsyncIterable<ResponseListItem> {
    const self = this;
    return {
      async *[Symbol.asyncIterator]() {
        const page = await self.listPage(params, options);
        yield* page.iterateAll();
      },
    };
  }

  retrieve(responseID: string, options?: RequestOptions): Promise<AgentResponse> {
    return this.http
      .request<AgentResponse>("GET", `${this.path}/${encodeURIComponent(responseID)}`, undefined, options)
      .then(addOutputText);
  }

  cancel(responseID: string, options?: RequestOptions): Promise<CancelResponse> {
    return this.http.request<CancelResponse>(
      "POST",
      `${this.path}/${encodeURIComponent(responseID)}/cancel`,
      undefined,
      options,
    );
  }

  listChildren(responseID: string, options?: RequestOptions): Promise<ListChildrenResponse> {
    return this.http.request<ListChildrenResponse>(
      "GET",
      `${this.path}/${encodeURIComponent(responseID)}/children`,
      undefined,
      options,
    );
  }

  listEvents(responseID: string, params: ListEventsParams = {}, options?: RequestOptions): Promise<ListEventsResponse> {
    return this.http.request<ListEventsResponse>(
      "GET",
      `${this.path}/${encodeURIComponent(responseID)}/events${buildQuery({
        after_sequence: params.after_sequence,
        view: params.view,
      })}`,
      undefined,
      options,
    );
  }

  retrieveVolume(responseID: string, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>(
      "GET",
      `${this.path}/${encodeURIComponent(responseID)}/volume`,
      undefined,
      options,
    );
  }
}
