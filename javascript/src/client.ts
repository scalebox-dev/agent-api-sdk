import { APIConnectionError } from "./errors.js";
import { readEnv } from "./internal/env.js";
import { HTTPClient } from "./internal/http.js";
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
  readonly baseURL: string;
  readonly timeout: number;
  readonly streamTimeout: number;
  readonly maxRetries: number;
  readonly defaultHeaders: Record<string, string>;
  readonly responses: ResponsesResource;
  readonly agent: ResponsesResource;
  readonly models: ModelsResource;
  readonly presets: PresetsResource;
  readonly tools: ToolsResource;
  readonly volumes: VolumesResource;
  readonly skills: SkillsResource;
  private readonly http: HTTPClient;

  constructor(options: ClientOptions = {}) {
    this.apiKey = options.apiKey ?? readEnv("AGENT_API_KEY");
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
      timeout: this.timeout,
      streamTimeout: this.streamTimeout,
      maxRetries: this.maxRetries,
      defaultHeaders: this.defaultHeaders,
      fetchImpl,
    });
    this.responses = new ResponsesResource(this.http, "/v1/responses");
    this.agent = new ResponsesResource(this.http, "/v1/agent");
    this.models = new ModelsResource(this.http);
    this.presets = new PresetsResource(this.http);
    this.tools = new ToolsResource(this.http);
    this.volumes = new VolumesResource(this.http);
    this.skills = new SkillsResource(this.http);
  }
}

export { VERSION };
