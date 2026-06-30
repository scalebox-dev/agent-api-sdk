import { APIConnectionError } from "./errors.js";
import { readEnv } from "./internal/env.js";
import { HTTPClient } from "./internal/http.js";
import { AuthResource } from "./resources/auth.js";
import { MemoriesResource } from "./resources/memories.js";
import { ModelsResource } from "./resources/models.js";
import { PresetsResource } from "./resources/presets.js";
import { ResponsesResource } from "./resources/responses.js";
import { SkillsResource } from "./resources/skills.js";
import { ToolsResource } from "./resources/tools.js";
import { VolumesResource } from "./resources/volumes.js";
import type { ClientOptions } from "./types/common.js";
import { VERSION } from "./version.js";

export const DEFAULT_TIMEOUT_MS = 600_000;
export const DEFAULT_STREAM_TIMEOUT_MS = 3_600_000;
export const DEFAULT_MAX_RETRIES = 2;

export class AgentAPI {
  readonly apiKey?: string;
  readonly apiKeyProvider?: ClientOptions["apiKeyProvider"];
  readonly baseURL: string;
  readonly timeout: number;
  readonly streamTimeout: number;
  readonly maxRetries: number;
  readonly defaultHeaders: Record<string, string>;
  readonly responses: ResponsesResource;
  readonly agent: ResponsesResource;
  readonly models: ModelsResource;
  readonly memories: MemoriesResource;
  readonly presets: PresetsResource;
  readonly tools: ToolsResource;
  readonly volumes: VolumesResource;
  readonly skills: SkillsResource;
  readonly auth: AuthResource;
  protected readonly http: HTTPClient;

  constructor(options: ClientOptions = {}) {
    this.apiKey = options.apiKey ?? readEnv("AGENT_API_KEY");
    this.apiKeyProvider = options.apiKeyProvider;
    this.baseURL = (options.baseURL ?? readEnv("AGENT_API_BASE_URL") ?? "https://api.agentsway.dev").replace(/\/+$/, "");
    this.timeout = options.timeout ?? DEFAULT_TIMEOUT_MS;
    this.streamTimeout = options.streamTimeout ?? DEFAULT_STREAM_TIMEOUT_MS;
    this.maxRetries = options.maxRetries ?? DEFAULT_MAX_RETRIES;
    this.defaultHeaders = options.defaultHeaders ?? {};
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) {
      throw new APIConnectionError("No fetch implementation is available");
    }
    this.http = new HTTPClient({
      baseURL: this.baseURL,
      apiKey: this.apiKey,
      apiKeyProvider: this.apiKeyProvider,
      timeout: this.timeout,
      streamTimeout: this.streamTimeout,
      maxRetries: this.maxRetries,
      defaultHeaders: this.defaultHeaders,
      fetchImpl,
    });
    this.responses = new ResponsesResource(this.http, "/v1/responses");
    this.agent = new ResponsesResource(this.http, "/v1/agent");
    this.models = new ModelsResource(this.http);
    this.memories = new MemoriesResource(this.http);
    this.presets = new PresetsResource(this.http);
    this.tools = new ToolsResource(this.http);
    this.volumes = new VolumesResource(this.http);
    this.skills = this.createSkillsResource();
    this.auth = new AuthResource(this.http);
  }

  protected createSkillsResource(): SkillsResource {
    return new SkillsResource(this.http);
  }

  startDeviceAuth = (...args: Parameters<AuthResource["startDeviceAuth"]>) => this.auth.startDeviceAuth(...args);
  pollDeviceAuth = (...args: Parameters<AuthResource["pollDeviceAuth"]>) => this.auth.pollDeviceAuth(...args);
  refreshBrowserSession = (...args: Parameters<AuthResource["refreshBrowserSession"]>) => this.auth.refreshBrowserSession(...args);
  waitForDeviceAuth = (...args: Parameters<AuthResource["waitForDeviceAuth"]>) => this.auth.waitForDeviceAuth(...args);
}

export { VERSION };
