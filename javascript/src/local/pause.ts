import type { FunctionTool } from "../types/tools.js";

export interface LocalPauseRequest {
  durationMs: number;
  reason?: string;
}

export interface LocalPauseResult extends Record<string, unknown> {
  ok: true;
  tool: string;
  action: "pause";
  requested_ms: number;
  elapsed_ms: number;
  status: "completed" | "cancelled";
  reason?: string;
  resume_message?: string;
}

export interface LocalPauseHandle {
  readonly request: LocalPauseRequest;
  resume(message?: string): void;
}

export interface LocalPauseToolRegistryOptions {
  maxDurationMs?: number;
  toolName?: string;
  onPauseStart?: (handle: LocalPauseHandle) => void;
  onPauseEnd?: (result: LocalPauseResult) => void;
}

export interface LocalPauseToolRegistry {
  readonly maxDurationMs: number;
  readonly toolName: string;
  definitions(): FunctionTool[];
  handlers(): Record<string, (args: Record<string, unknown>) => Promise<Record<string, unknown>>>;
  execute(name: string, args: Record<string, unknown>, context?: { signal?: AbortSignal }): Promise<Record<string, unknown>>;
  requiresApproval(name: string, args?: Record<string, unknown>): boolean;
}

const defaultMaxDurationMs = 5 * 60 * 1000;

export function createLocalPauseToolRegistry(options: LocalPauseToolRegistryOptions = {}): LocalPauseToolRegistry {
  const toolName = options.toolName ?? "local_pause";
  const maxDurationMs = Math.max(1, Math.floor(options.maxDurationMs ?? defaultMaxDurationMs));
  const definition = localPauseToolDefinition(toolName, { maxDurationMs });

  return {
    maxDurationMs,
    toolName,
    definitions: () => [{ ...definition }],
    handlers: () => ({ [toolName]: (args: Record<string, unknown>) => executeLocalPause(toolName, args, { maxDurationMs, ...options }) }),
    execute: async (name, args, context = {}) => {
      if (name !== toolName) {
        throw new Error(`unknown local pause tool: ${name}`);
      }
      return await executeLocalPause(toolName, args, { maxDurationMs, signal: context.signal, ...options });
    },
    requiresApproval: (name) => name === toolName && false,
  };
}

export function localPauseToolDefinition(name = "local_pause", options: { maxDurationMs?: number } = {}): FunctionTool {
  return {
    type: "function",
    name,
    description: localPauseToolInstructions(options),
    parameters: localPauseToolParameters(options),
    strict: false,
  };
}

export function localPauseToolInstructions(options: { maxDurationMs?: number } = {}): string {
  const maxDurationMs = Math.max(1, Math.floor(options.maxDurationMs ?? defaultMaxDurationMs));
  return [
    "Pause the local agentic workflow for a bounded amount of time, then continue automatically.",
    "Use this only when waiting for external state such as CI, deployment rollout, rate-limit cooldown, file sync, or another asynchronous process.",
    "The pause is local runtime control, not reasoning time. Keep the reason concrete.",
    `Maximum duration: ${maxDurationMs} ms. The user may resume the pause early; early resume returns status=cancelled with elapsed_ms.`,
  ].join(" ");
}

function localPauseToolParameters(options: { maxDurationMs?: number } = {}): Record<string, unknown> {
  const maxDurationMs = Math.max(1, Math.floor(options.maxDurationMs ?? defaultMaxDurationMs));
  return {
    type: "object",
    properties: {
      duration_ms: {
        type: "integer",
        minimum: 1,
        maximum: maxDurationMs,
        description: "How long to wait before continuing automatically, in milliseconds.",
      },
      reason: {
        type: "string",
        description: "Short reason for the wait, such as the external state being awaited.",
      },
    },
    required: ["duration_ms"],
    additionalProperties: false,
  };
}

async function executeLocalPause(
  toolName: string,
  args: Record<string, unknown>,
  options: LocalPauseToolRegistryOptions & { maxDurationMs: number; signal?: AbortSignal },
): Promise<LocalPauseResult> {
  const request = pauseRequest(args, options.maxDurationMs);
  const startedAt = Date.now();
  let settled = false;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let abortHandler: (() => void) | null = null;

  return await new Promise<LocalPauseResult>((resolve, reject) => {
    const finish = (status: LocalPauseResult["status"], resumeMessage?: string) => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      if (abortHandler) options.signal?.removeEventListener("abort", abortHandler);
      const result: LocalPauseResult = {
        ok: true,
        tool: toolName,
        action: "pause",
        requested_ms: request.durationMs,
        elapsed_ms: Math.max(0, Date.now() - startedAt),
        status,
        ...(request.reason ? { reason: request.reason } : {}),
        ...(resumeMessage ? { resume_message: resumeMessage } : {}),
      };
      options.onPauseEnd?.(result);
      resolve(result);
    };
    abortHandler = () => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      reject(new Error("local_pause aborted"));
    };
    if (options.signal?.aborted) {
      abortHandler();
      return;
    }
    options.signal?.addEventListener("abort", abortHandler);
    options.onPauseStart?.({
      request,
      resume(message?: string) {
        finish("cancelled", message);
      },
    });
    timer = setTimeout(() => finish("completed"), request.durationMs);
  });
}

function pauseRequest(args: Record<string, unknown>, maxDurationMs: number): LocalPauseRequest {
  const raw = args.duration_ms ?? args.durationMs;
  if (typeof raw !== "number" || !Number.isFinite(raw)) {
    throw new Error("local_pause duration_ms must be a finite number");
  }
  const durationMs = Math.floor(raw);
  if (durationMs < 1) {
    throw new Error("local_pause duration_ms must be at least 1");
  }
  if (durationMs > maxDurationMs) {
    throw new Error(`local_pause duration_ms must be <= ${maxDurationMs}`);
  }
  const reason = typeof args.reason === "string" && args.reason.trim() ? args.reason.trim() : undefined;
  return { durationMs, reason };
}
