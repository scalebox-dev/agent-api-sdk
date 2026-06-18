import type { HTTPClient } from "../internal/http.js";
import type {
  ApprovedDeviceAuth,
  AuthSession,
  DeviceAuthPollResult,
  DeviceAuthStart,
  PollDeviceAuthParams,
  RefreshBrowserSessionParams,
  StartDeviceAuthParams,
  WaitForDeviceAuthParams,
} from "../types/auth.js";
import type { RequestOptions } from "../types/common.js";

const defaultDevicePollIntervalSeconds = 5;

export class AuthResource {
  constructor(private readonly http: HTTPClient) {}

  startDeviceAuth(params: StartDeviceAuthParams = {}, options: RequestOptions = {}): Promise<DeviceAuthStart> {
    return this.http.request<DeviceAuthStart>("POST", "/v1/auth/device/start", params, options);
  }

  pollDeviceAuth(params: PollDeviceAuthParams, options: RequestOptions = {}): Promise<DeviceAuthPollResult> {
    requireDeviceCode(params.device_code);
    return this.http.request<DeviceAuthPollResult>("POST", "/v1/auth/device/poll", params, options);
  }

  refreshBrowserSession(params: RefreshBrowserSessionParams, options: RequestOptions = {}): Promise<AuthSession> {
    requireRefreshToken(params.refresh_token);
    return this.http.request<AuthSession>("POST", "/v1/auth/refresh", {}, {
      ...options,
      headers: {
        ...options.headers,
        Cookie: `agent_api_refresh=${encodeURIComponent(params.refresh_token)}`,
      },
    });
  }

  async waitForDeviceAuth(params: WaitForDeviceAuthParams, options: RequestOptions = {}): Promise<ApprovedDeviceAuth> {
    requireDeviceCode(params.device_code);
    const startedAt = Date.now();
    for (;;) {
      throwIfAborted(params.signal);
      const result = await this.pollDeviceAuth({ device_code: params.device_code }, options);
      params.on_poll?.(result);
      if (result.status === "approved" && result.access_token && result.refresh_token) {
        return result as ApprovedDeviceAuth;
      }
      if (result.status === "expired" || result.status === "consumed") {
        throw new DeviceAuthFlowError(result.message || `Device auth ${result.status}`, result);
      }
      const timeoutMs = effectiveTimeoutMs(params, result);
      if (timeoutMs !== undefined && Date.now() - startedAt >= timeoutMs) {
        throw new DeviceAuthFlowError("Device auth timed out", result);
      }
      await sleep(pollDelayMs(params, result), params.signal);
    }
  }
}

export function browserAuthSessionExpiresWithin(
  session: Pick<AuthSession, "access_token_expires_at">,
  windowMs = 60_000,
  nowMs = Date.now(),
): boolean {
  return session.access_token_expires_at * 1000 - nowMs <= windowMs;
}

export class DeviceAuthFlowError extends Error {
  readonly result?: DeviceAuthPollResult;

  constructor(message: string, result?: DeviceAuthPollResult) {
    super(message);
    this.name = "DeviceAuthFlowError";
    this.result = result;
    Object.setPrototypeOf(this, DeviceAuthFlowError.prototype);
  }
}

function requireDeviceCode(deviceCode: string) {
  if (!deviceCode || !deviceCode.trim()) {
    throw new TypeError("device_code is required");
  }
}

function requireRefreshToken(refreshToken: string) {
  if (!refreshToken || !refreshToken.trim()) {
    throw new TypeError("refresh_token is required");
  }
}

function pollDelayMs(params: WaitForDeviceAuthParams, result: DeviceAuthPollResult) {
  const seconds = params.interval_seconds ?? result.interval_seconds ?? defaultDevicePollIntervalSeconds;
  return Math.max(1, seconds) * 1000;
}

function effectiveTimeoutMs(params: WaitForDeviceAuthParams, result: DeviceAuthPollResult) {
  if (params.timeout_ms !== undefined) {
    return Math.max(0, params.timeout_ms);
  }
  const expiresAt = result.expires_at;
  if (!expiresAt) return undefined;
  return Math.max(0, expiresAt * 1000 - Date.now());
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  if (!signal) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
  throwIfAborted(signal);
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(cleanupResolve, ms);
    signal.addEventListener("abort", cleanupReject, { once: true });
    function cleanupResolve() {
      signal?.removeEventListener("abort", cleanupReject);
      resolve();
    }
    function cleanupReject() {
      clearTimeout(timeout);
      reject(abortError());
    }
  });
}

function throwIfAborted(signal?: AbortSignal) {
  if (signal?.aborted) {
    throw abortError();
  }
}

function abortError() {
  return new DOMException("Device auth wait aborted", "AbortError");
}
