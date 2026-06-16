import {
  APIConnectionError,
  APIError,
  isRetryableStatus,
  parseResponseError,
  RateLimitError,
} from "../errors.js";
import { parseSSE } from "../streaming.js";
import type { RequestOptions } from "../types/common.js";
import { USER_AGENT } from "../version.js";

export interface HTTPClientOptions {
  baseURL: string;
  apiKey?: string;
  timeout: number;
  streamTimeout: number;
  maxRetries: number;
  defaultHeaders: Record<string, string>;
  fetchImpl: typeof fetch;
}

export class HTTPClient {
  constructor(private readonly options: HTTPClientOptions) {}

  async request<T>(method: string, path: string, body?: unknown, options: RequestOptions = {}): Promise<T> {
    const response = await this.fetchWithRetry(method, path, body, options, false, false);
    return (await response.json()) as T;
  }

  async requestRaw<T>(method: string, path: string, body: BodyInit, options: RequestOptions = {}): Promise<T> {
    const response = await this.fetchWithRetry(method, path, body, options, false, true);
    return (await response.json()) as T;
  }

  async requestVoid(method: string, path: string, options: RequestOptions = {}): Promise<void> {
    await this.fetchWithRetry(method, path, undefined, options, false, false);
  }

  async requestBinary(
    method: string,
    path: string,
    options: RequestOptions = {},
  ): Promise<{ body: ArrayBuffer; headers: Headers }> {
    const response = await this.fetchWithRetry(method, path, undefined, options, false, false);
    return { body: await response.arrayBuffer(), headers: response.headers };
  }

  async stream<T>(path: string, body: unknown, options: RequestOptions = {}): Promise<AsyncIterable<T>> {
    const response = await this.fetchWithRetry("POST", path, body, options, true, false);
    return parseSSE<T>(response);
  }

  private async fetchWithRetry(
    method: string,
    path: string,
    body: unknown,
    options: RequestOptions,
    stream: boolean,
    rawBody: boolean,
  ): Promise<Response> {
    const maxRetries = options.maxRetries ?? this.options.maxRetries;
    let attempt = 0;

    for (;;) {
      try {
        return await this.fetchOnce(method, path, body, options, stream, rawBody);
      } catch (error) {
        if (!(error instanceof APIError) || attempt >= maxRetries) {
          throw error;
        }
        const retryable =
          error instanceof APIConnectionError ||
          (error.status !== undefined && isRetryableStatus(error.status));
        if (!retryable) {
          throw error;
        }
        attempt += 1;
        const delayMs = retryDelayMs(error, attempt);
        await sleep(delayMs);
      }
    }
  }

  private async fetchOnce(
    method: string,
    path: string,
    body: unknown,
    options: RequestOptions,
    stream: boolean,
    rawBody: boolean,
  ): Promise<Response> {
    const controller = new AbortController();
    const timeout = options.timeout ?? (stream ? this.options.streamTimeout : this.options.timeout);
    const timeoutID = setTimeout(() => controller.abort(), timeout);

    try {
      const headers: Record<string, string> = {
        Accept: stream ? "text/event-stream" : "application/json",
        "User-Agent": USER_AGENT,
        ...this.options.defaultHeaders,
        ...options.headers,
      };
      if (body !== undefined && !rawBody) {
        headers["Content-Type"] = "application/json";
      }
      if (this.options.apiKey) {
        headers.Authorization = `Bearer ${this.options.apiKey}`;
      }

      const response = await this.options.fetchImpl(`${this.options.baseURL}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : rawBody ? (body as BodyInit) : JSON.stringify(body),
        signal: controller.signal,
      });
      if (!response.ok) {
        throw await parseResponseError(response);
      }
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      if (error instanceof Error && error.name === "AbortError") {
        throw new APIConnectionError(`Request timed out after ${timeout}ms`, error);
      }
      throw new APIConnectionError("Request failed", error);
    } finally {
      clearTimeout(timeoutID);
    }
  }
}

function retryDelayMs(error: APIError, attempt: number): number {
  if (error instanceof RateLimitError && error.retryAfterMs !== undefined) {
    return error.retryAfterMs;
  }
  const base = 250;
  const cap = 8000;
  const exponential = Math.min(cap, base * 2 ** (attempt - 1));
  const jitter = Math.floor(Math.random() * 100);
  return exponential + jitter;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
