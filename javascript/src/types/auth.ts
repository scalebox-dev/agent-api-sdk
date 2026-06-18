export type DeviceAuthStatus = "pending" | "approved" | "expired" | "consumed" | string;

export interface DeviceAuthStart {
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete: string;
  expires_at: number;
  interval_seconds: number;
}

export interface AuthSession {
  access_token: string;
  refresh_token: string;
  token_type?: string;
  access_token_expires_at: number;
  refresh_token_expires_at?: number;
  user_id: string;
  workspace_id: string;
  workspace_name?: string;
  workspace_role: string;
  scopes: string[];
}

export interface RefreshBrowserSessionParams {
  refresh_token: string;
}

export interface DeviceAuthPollResult {
  status: DeviceAuthStatus;
  message?: string;
  interval_seconds?: number;
  expires_at?: number;
  access_token?: string;
  refresh_token?: string;
  token_type?: string;
  access_token_expires_at?: number;
  refresh_token_expires_at?: number;
  user_id?: string;
  workspace_id?: string;
  workspace_name?: string;
  workspace_role?: string;
  scopes?: string[];
}

export type ApprovedDeviceAuth = DeviceAuthPollResult & {
  status: "approved";
  access_token: string;
  refresh_token: string;
  access_token_expires_at: number;
  user_id: string;
  workspace_id: string;
  workspace_role: string;
  scopes: string[];
};

export interface StartDeviceAuthParams {
  client_name?: string;
}

export interface PollDeviceAuthParams {
  device_code: string;
}

export interface WaitForDeviceAuthParams extends PollDeviceAuthParams {
  /** Polling interval in seconds. Defaults to the server interval or 5 seconds. */
  interval_seconds?: number;
  /** Overall wait timeout in milliseconds. Defaults to the challenge expiry when present. */
  timeout_ms?: number;
  /** Optional cancellation signal for CLI/Desktop integrations. */
  signal?: AbortSignal;
  /** Called after every poll response, including pending responses. */
  on_poll?: (result: DeviceAuthPollResult) => void;
}
