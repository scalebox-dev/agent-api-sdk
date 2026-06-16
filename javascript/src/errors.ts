export interface APIErrorBody {
  error?: {
    message?: string;
    code?: string;
    type?: string;
  };
}

export class APIError extends Error {
  readonly status?: number;
  readonly headers?: Headers;
  readonly body?: unknown;
  readonly code?: string;
  readonly errorType?: string;
  readonly requestID?: string;

  constructor(
    message: string,
    options: {
      status?: number;
      headers?: Headers;
      body?: unknown;
      code?: string;
      errorType?: string;
      requestID?: string;
    } = {},
  ) {
    super(message);
    this.name = "APIError";
    this.status = options.status;
    this.headers = options.headers;
    this.body = options.body;
    this.code = options.code;
    this.errorType = options.errorType;
    this.requestID = options.requestID;
  }
}

export class APIConnectionError extends APIError {
  constructor(message: string, cause?: unknown) {
    super(message, { body: cause });
    this.name = "APIConnectionError";
  }
}

export class APIStatusError extends APIError {
  readonly status: number;

  constructor(
    message: string,
    status: number,
    headers: Headers,
    body: unknown,
    options: { code?: string; errorType?: string; requestID?: string } = {},
  ) {
    super(message, { status, headers, body, ...options });
    this.name = "APIStatusError";
    this.status = status;
  }
}

export class AuthenticationError extends APIStatusError {
  constructor(message: string, status: number, headers: Headers, body: unknown, options = {}) {
    super(message, status, headers, body, options);
    this.name = "AuthenticationError";
  }
}

export class PermissionDeniedError extends APIStatusError {
  constructor(message: string, status: number, headers: Headers, body: unknown, options = {}) {
    super(message, status, headers, body, options);
    this.name = "PermissionDeniedError";
  }
}

export class NotFoundError extends APIStatusError {
  constructor(message: string, status: number, headers: Headers, body: unknown, options = {}) {
    super(message, status, headers, body, options);
    this.name = "NotFoundError";
  }
}

export class BadRequestError extends APIStatusError {
  constructor(message: string, status: number, headers: Headers, body: unknown, options = {}) {
    super(message, status, headers, body, options);
    this.name = "BadRequestError";
  }
}

export class RateLimitError extends APIStatusError {
  readonly retryAfterMs?: number;

  constructor(
    message: string,
    status: number,
    headers: Headers,
    body: unknown,
    retryAfterMs?: number,
    options: { code?: string; errorType?: string; requestID?: string } = {},
  ) {
    super(message, status, headers, body, options);
    this.name = "RateLimitError";
    this.retryAfterMs = retryAfterMs;
  }
}

export class InternalServerError extends APIStatusError {
  constructor(message: string, status: number, headers: Headers, body: unknown, options = {}) {
    super(message, status, headers, body, options);
    this.name = "InternalServerError";
  }
}

export function requestIDFromHeaders(headers: Headers): string | undefined {
  return headers.get("x-request-id") ?? headers.get("request-id") ?? undefined;
}

export function retryAfterMsFromHeaders(headers: Headers): number | undefined {
  const raw = headers.get("retry-after");
  if (!raw) {
    return undefined;
  }
  const seconds = Number(raw);
  if (Number.isFinite(seconds)) {
    return Math.max(0, seconds * 1000);
  }
  const date = Date.parse(raw);
  if (Number.isFinite(date)) {
    return Math.max(0, date - Date.now());
  }
  return undefined;
}

function errorFields(body: unknown): { message: string; code?: string; errorType?: string } {
  if (typeof body === "object" && body !== null && "error" in body) {
    const err = (body as APIErrorBody).error;
    if (err && typeof err.message === "string") {
      return { message: err.message, code: err.code, errorType: err.type };
    }
  }
  return { message: "" };
}

export async function parseResponseError(response: Response): Promise<APIStatusError> {
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    body = await response.text().catch(() => "");
  }

  const { message, code, errorType } = errorFields(body);
  const requestID = requestIDFromHeaders(response.headers);
  const finalMessage = message || `API request failed with status ${response.status}`;
  const common = { code, errorType, requestID };

  switch (response.status) {
    case 400:
      return new BadRequestError(finalMessage, response.status, response.headers, body, common);
    case 401:
      return new AuthenticationError(finalMessage, response.status, response.headers, body, common);
    case 403:
      return new PermissionDeniedError(finalMessage, response.status, response.headers, body, common);
    case 404:
      return new NotFoundError(finalMessage, response.status, response.headers, body, common);
    case 429:
      return new RateLimitError(
        finalMessage,
        response.status,
        response.headers,
        body,
        retryAfterMsFromHeaders(response.headers),
        common,
      );
    default:
      if (response.status >= 500) {
        return new InternalServerError(finalMessage, response.status, response.headers, body, common);
      }
      return new APIStatusError(finalMessage, response.status, response.headers, body, common);
  }
}

export function isRetryableStatus(status: number): boolean {
  return status === 408 || status === 409 || status === 429 || status === 500 || status === 502 || status === 503 || status === 504;
}
