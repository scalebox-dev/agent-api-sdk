import type { AgentCapabilityPreference } from "./common.js";

export interface ModelCapabilities {
  provider?: string;
  supports_streaming?: boolean;
  supports_tools?: boolean;
  supports_json_schema?: boolean;
  supports_reasoning?: boolean;
  context_window?: number;
  pricing?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}

export interface Model {
  id: string;
  object: "model";
  owned_by: string;
  capabilities?: ModelCapabilities;
}

export interface ListModelsResponse {
  object: "list";
  data: Model[];
}

export interface PresetPolicy {
  plan_mode_preference?: AgentCapabilityPreference;
  sub_agent_preference?: AgentCapabilityPreference;
  allowed_tools?: string[];
  max_steps?: number;
}

export interface Preset {
  preset: string;
  prompt_version?: string;
  preset_metadata?: Record<string, unknown>;
  policy?: PresetPolicy;
  max_output_tokens?: number;
  default_model?: string;
  model_chain?: string[];
}

export interface ListPresetsResponse {
  object: "list";
  data: Preset[];
}
