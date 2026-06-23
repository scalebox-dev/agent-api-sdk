import { spawn, spawnSync } from "node:child_process";
import path from "node:path";

import type { FunctionTool } from "../types/tools.js";
import type { LocalWorkdir } from "./core.js";

export type LocalShellAccessMode = "approval" | "full";
/**
 * Requested isolation policy for local shell execution.
 * - none: direct host execution.
 * - auto: use an isolating runner when the host app provides one; otherwise fall back to direct execution.
 * - required: fail closed unless the configured runner reports real isolation.
 */
export type LocalShellIsolationMode = "none" | "auto" | "required";

export interface LocalShellIsolationGuarantees {
  filesystem: "none" | "workdir-mounted" | "policy-enforced";
  network: "allowed" | "blocked" | "configurable";
  user: "host-user" | "unprivileged-user" | "namespace-user";
  process: "host-process-tree" | "child-contained" | "pid-namespace";
  resources: "none" | "timeout-only" | "cpu-memory-limits";
}

export interface LocalShellIsolationResourceOptions {
  memoryMb?: number;
  cpuCount?: number;
}

export interface LocalShellIsolationOptions {
  filesystem?: "host" | "workdir-readonly" | "workdir-readwrite";
  network?: "allowed" | "blocked";
  env?: "inherit" | "minimal";
  resources?: LocalShellIsolationResourceOptions;
}

export interface LocalShellIsolationStatus {
  executor: "direct" | "isolator";
  driver: string;
  isolated: boolean;
  fallback: boolean;
  requested: Required<LocalShellIsolationOptions> & { resources: LocalShellIsolationResourceOptions };
  guarantees: LocalShellIsolationGuarantees;
  warnings: string[];
}

export interface LocalShellIsolatorStatusResult {
  version?: string;
  driver: string;
  status: LocalShellIsolationStatus;
  drivers?: Array<{
    name: string;
    platform: string;
    available: boolean;
    warnings?: string[];
  }>;
}

export interface LocalShellRequest {
  command: string;
  description?: string;
  workdir?: string;
  timeoutMs?: number;
  env?: Record<string, string | undefined>;
}

export interface LocalShellContext {
  signal?: AbortSignal;
}

export interface LocalShellResult {
  ok: boolean;
  command: string;
  description?: string;
  cwd: string;
  exit_code: number | null;
  signal: NodeJS.Signals | null;
  stdout: string;
  stderr: string;
  output: string;
  duration_ms: number;
  timed_out: boolean;
  truncated: boolean;
  shell_isolation: LocalShellIsolationStatus;
}

export interface LocalCommandRunner {
  readonly isolationStatus?: LocalShellIsolationStatus;
  run(request: LocalShellRequest, context?: LocalShellContext): Promise<LocalShellResult>;
}

export interface HostLocalShellRunnerOptions {
  cwd?: string;
  shell?: string | boolean;
  timeoutMs?: number;
  maxOutputBytes?: number;
  env?: NodeJS.ProcessEnv;
  isolationOptions?: LocalShellIsolationOptions;
}

export interface IsolatorLocalShellRunnerOptions {
  executablePath?: string;
  driver?: "auto" | "direct" | "bwrap" | "sandbox-exec" | "windows-job" | string;
  cwd?: string;
  timeoutMs?: number;
  maxOutputBytes?: number;
  isolationOptions?: LocalShellIsolationOptions;
}

export class HostLocalShellRunner implements LocalCommandRunner {
  readonly cwd: string;
  readonly shell: string | boolean;
  readonly timeoutMs: number;
  readonly maxOutputBytes: number;
  readonly env: NodeJS.ProcessEnv;
  readonly isolationStatus: LocalShellIsolationStatus;

  constructor(options: HostLocalShellRunnerOptions = {}) {
    this.cwd = path.resolve(options.cwd ?? process.cwd());
    this.shell = options.shell ?? true;
    this.timeoutMs = positiveInteger(options.timeoutMs, 2 * 60 * 1000, "timeoutMs");
    this.maxOutputBytes = positiveInteger(options.maxOutputBytes, 128 * 1024, "maxOutputBytes");
    this.env = { ...process.env, ...(options.env ?? {}) };
    this.isolationStatus = directIsolationStatus(false, options.isolationOptions);
  }

  async run(request: LocalShellRequest, context: LocalShellContext = {}): Promise<LocalShellResult> {
    const command = requiredString(request.command, "command");
    const cwd = resolveContainedPath(this.cwd, request.workdir);
    const timeoutMs = request.timeoutMs == null
      ? this.timeoutMs
      : positiveInteger(request.timeoutMs, this.timeoutMs, "timeoutMs");
    const started = Date.now();
    const stdout = new BoundedText(this.maxOutputBytes);
    const stderr = new BoundedText(this.maxOutputBytes);
    let timedOut = false;

    return await new Promise<LocalShellResult>((resolve, reject) => {
      const child = spawn(command, [], {
        cwd,
        env: { ...this.env, ...(request.env ?? {}) },
        shell: this.shell,
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
      });

      let settled = false;
      const finish = (exitCode: number | null, signal: NodeJS.Signals | null) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        context.signal?.removeEventListener("abort", abort);
        const stdoutText = stdout.text();
        const stderrText = stderr.text();
        const output = [stdoutText, stderrText].filter(Boolean).join(stderrText ? "\n" : "");
        resolve({
          ok: true,
          command,
          description: request.description,
          cwd,
          exit_code: exitCode,
          signal,
          stdout: stdoutText,
          stderr: stderrText,
          output: output || "(no output)",
          duration_ms: Date.now() - started,
          timed_out: timedOut,
          truncated: stdout.truncated || stderr.truncated,
          shell_isolation: this.isolationStatus,
        });
      };

      const kill = () => {
        if (child.killed) return;
        child.kill(process.platform === "win32" ? undefined : "SIGTERM");
        setTimeout(() => {
          if (!child.killed) child.kill(process.platform === "win32" ? undefined : "SIGKILL");
        }, 3000).unref();
      };

      const abort = () => {
        kill();
      };

      const timer = setTimeout(() => {
        timedOut = true;
        kill();
      }, timeoutMs);
      timer.unref();

      if (context.signal?.aborted) {
        abort();
      } else {
        context.signal?.addEventListener("abort", abort, { once: true });
      }

      child.stdout?.on("data", (chunk: Buffer) => stdout.append(chunk));
      child.stderr?.on("data", (chunk: Buffer) => stderr.append(chunk));
      child.on("error", (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        context.signal?.removeEventListener("abort", abort);
        reject(error);
      });
      child.on("close", finish);
    });
  }
}

export class IsolatorLocalShellRunner implements LocalCommandRunner {
  readonly executablePath: string;
  readonly driver: string;
  readonly cwd: string;
  readonly timeoutMs: number;
  readonly maxOutputBytes: number;
  readonly isolationOptions: LocalShellIsolationOptions;
  readonly isolationStatus: LocalShellIsolationStatus;
  readonly statusResult: LocalShellIsolatorStatusResult;

  constructor(options: IsolatorLocalShellRunnerOptions = {}) {
    this.executablePath = resolveIsolatorExecutablePath(options.executablePath);
    this.driver = options.driver ?? "auto";
    this.cwd = path.resolve(options.cwd ?? process.cwd());
    this.timeoutMs = positiveInteger(options.timeoutMs, 2 * 60 * 1000, "timeoutMs");
    this.maxOutputBytes = positiveInteger(options.maxOutputBytes, 128 * 1024, "maxOutputBytes");
    this.isolationOptions = options.isolationOptions ?? {};
    this.statusResult = isolatorStatusSync(this.executablePath, this.driver);
    this.isolationStatus = this.statusResult.status;
  }

  async run(request: LocalShellRequest, context: LocalShellContext = {}): Promise<LocalShellResult> {
    const command = requiredString(request.command, "command");
    const cwd = resolveContainedPath(this.cwd, request.workdir);
    const timeoutMs = request.timeoutMs == null
      ? this.timeoutMs
      : positiveInteger(request.timeoutMs, this.timeoutMs, "timeoutMs");
    const response = await isolatorRequest(this.executablePath, this.driver, {
      id: `run_${Date.now().toString(36)}`,
      method: "run",
      params: {
        command,
        description: request.description,
        cwd,
        timeout_ms: timeoutMs,
        max_output_bytes: this.maxOutputBytes,
        env: request.env,
        isolation: this.isolationOptions,
      },
    }, context);
    return isolatorRunResult(response.result);
  }
}

export interface LocalShellToolRegistryOptions {
  accessMode?: LocalShellAccessMode;
  isolation?: LocalShellIsolationMode;
  isolationOptions?: LocalShellIsolationOptions;
  isolator?: IsolatorLocalShellRunnerOptions | boolean;
  toolName?: string;
  runner?: LocalCommandRunner;
  cwd?: string;
  workdir?: LocalWorkdir;
  shell?: string | boolean;
  timeoutMs?: number;
  maxOutputBytes?: number;
}

export interface LocalShellToolPresentationOptions {
  accessMode?: LocalShellAccessMode;
  isolationStatus?: LocalShellIsolationStatus;
  isolationOptions?: LocalShellIsolationOptions;
  cwd?: string;
  shell?: string | boolean;
  platform?: NodeJS.Platform;
  timeoutMs?: number;
  maxOutputBytes?: number;
}

export interface LocalShellToolRegistry {
  readonly accessMode: LocalShellAccessMode;
  readonly toolName: string;
  definitions(): FunctionTool[];
  handlers(): Record<string, (args: Record<string, unknown>) => Promise<Record<string, unknown>>>;
  execute(name: string, args: Record<string, unknown>, context?: LocalShellContext): Promise<Record<string, unknown>>;
  requiresApproval(name: string, args?: Record<string, unknown>): boolean;
}

export class LocalShellDriver {
  readonly accessMode: LocalShellAccessMode;
  readonly runner: LocalCommandRunner;
  readonly isolationStatus: LocalShellIsolationStatus;

  constructor(options: LocalShellToolRegistryOptions = {}) {
    const cwd = options.workdir?.root ?? options.cwd;
    this.accessMode = options.accessMode ?? "approval";
    this.runner = resolveShellRunner(options, cwd);
    this.isolationStatus = this.runner.isolationStatus ?? directIsolationStatus(options.isolation === "auto", options.isolationOptions);
  }

  async dispatch(args: Record<string, unknown>, context: LocalShellContext = {}): Promise<Record<string, unknown>> {
    const request = shellRequest(args);
    if (this.accessMode !== "full") {
      return shellApprovalRequired(request, this.isolationStatus);
    }
    return shellToolResult(await this.runner.run(request, context));
  }

  requiresApproval(): boolean {
    return this.accessMode !== "full";
  }
}

function resolveShellRunner(options: LocalShellToolRegistryOptions, cwd: string | undefined): LocalCommandRunner {
  if (options.runner) {
    if (options.isolation === "required" && options.runner.isolationStatus?.isolated !== true) {
      throw new Error("local_shell isolation is required, but the configured runner does not report isolation");
    }
    return options.runner;
  }
  let isolatorFallbackWarning: string | undefined;
  if (options.isolation === "auto" || options.isolation === "required" || options.isolator) {
    const isolatorOptions = typeof options.isolator === "object" ? options.isolator : {};
    try {
      const runner = new IsolatorLocalShellRunner({
        cwd,
        timeoutMs: options.timeoutMs,
        maxOutputBytes: options.maxOutputBytes,
        isolationOptions: options.isolationOptions,
        ...isolatorOptions,
      });
      if (options.isolation === "required" && runner.isolationStatus.isolated !== true) {
        throw new Error(`local_shell isolation is required, but agent-isolator selected non-isolated driver ${runner.isolationStatus.driver}`);
      }
      if (options.isolation !== "none" || options.isolator) {
        return runner;
      }
    } catch (error) {
      if (options.isolation === "required") {
        throw error;
      }
      isolatorFallbackWarning = errorMessage(error);
    }
  }
  const runner = new HostLocalShellRunner({
    cwd,
    shell: options.shell,
    timeoutMs: options.timeoutMs,
    maxOutputBytes: options.maxOutputBytes,
    isolationOptions: options.isolationOptions,
  });
  if (options.isolation === "auto") {
    let status = directIsolationStatus(true, options.isolationOptions);
    if (isolatorFallbackWarning) {
      status = {
        ...status,
        warnings: [...status.warnings, `Isolator unavailable: ${isolatorFallbackWarning}`],
      };
    }
    return withIsolationStatus(runner, status);
  }
  return runner;
}

function resolveIsolatorExecutablePath(explicitPath: string | undefined): string {
  const configured = (explicitPath?.trim() || process.env.AGENT_ISOLATOR_PATH?.trim() || "");
  if (!configured) {
    throw new Error("agent-isolator executable path is not configured; pass isolator.executablePath or set AGENT_ISOLATOR_PATH");
  }
  return configured;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function createLocalShellToolRegistry(options: LocalShellToolRegistryOptions = {}): LocalShellToolRegistry {
  const toolName = options.toolName ?? "local_shell";
  const driver = new LocalShellDriver(options);
  const hostRunner = driver.runner instanceof HostLocalShellRunner ? driver.runner : undefined;
  const isolatorRunner = driver.runner instanceof IsolatorLocalShellRunner ? driver.runner : undefined;
  const definition = localShellToolDefinition(toolName, {
    accessMode: driver.accessMode,
    cwd: options.workdir?.root ?? hostRunner?.cwd ?? isolatorRunner?.cwd ?? options.cwd,
    shell: hostRunner?.shell ?? options.shell,
    timeoutMs: hostRunner?.timeoutMs ?? isolatorRunner?.timeoutMs ?? options.timeoutMs,
    maxOutputBytes: hostRunner?.maxOutputBytes ?? isolatorRunner?.maxOutputBytes ?? options.maxOutputBytes,
    isolationStatus: driver.isolationStatus,
    isolationOptions: options.isolationOptions,
  });

  return {
    accessMode: driver.accessMode,
    toolName,
    definitions: () => [{ ...definition }],
    handlers: () => ({ [toolName]: (args: Record<string, unknown>) => driver.dispatch(args) }),
    execute: async (name: string, args: Record<string, unknown>, context: LocalShellContext = {}) => {
      if (name !== toolName) {
        throw new Error(`unknown local shell tool: ${name}`);
      }
      return await driver.dispatch(args, context);
    },
    requiresApproval: (name: string) => name === toolName && driver.requiresApproval(),
  };
}

export function localShellToolDefinition(
  name = "local_shell",
  options: LocalShellToolPresentationOptions = {},
): FunctionTool {
  return {
    type: "function",
    name,
    description: localShellToolDescription(options),
    parameters: localShellToolParameters(),
    strict: false,
  };
}

export function localShellToolInstructions(options: LocalShellToolPresentationOptions = {}): string {
  return localShellToolDescription(options);
}

function localShellToolDescription(options: LocalShellToolPresentationOptions = {}): string {
  const platform = options.platform ?? process.platform;
  const shell = shellDisplayName(options.shell);
  const accessMode = options.accessMode ?? "approval";
  const isolation = options.isolationStatus ?? directIsolationStatus(false, options.isolationOptions);
  const cwd = options.cwd ? path.resolve(options.cwd) : undefined;
  const timeoutMs = options.timeoutMs ?? 2 * 60 * 1000;
  const maxOutputBytes = options.maxOutputBytes ?? 128 * 1024;
  return [
    "Run a local shell command through one model-facing primitive.",
    "Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
    `Execution environment: platform=${platform}; shell=${shell}; access_mode=${accessMode}; isolation_driver=${isolation.driver}; isolated=${isolation.isolated}; fallback=${isolation.fallback}; default_timeout_ms=${timeoutMs}; max_output_bytes=${maxOutputBytes}.`,
    isolationRequestDescription(isolation.requested),
    isolation.warnings.length > 0 ? `Isolation warning: ${isolation.warnings.join(" ")}` : "",
    cwd ? `Default cwd: ${cwd}. Relative command paths resolve from this cwd unless workdir is set.` : "Relative command paths resolve from the configured cwd unless workdir is set.",
    "The workdir parameter must be a relative child path of the configured cwd. Use workdir instead of cd when possible.",
    "Absolute paths inside the command are permitted by the host OS if the user/process has permission; this tool is not a filesystem sandbox or security sandbox.",
    "Captured stdout/stderr may be truncated when output exceeds the advertised max_output_bytes.",
    "In approval mode, calls return requires_approval instead of executing. In full mode, commands execute immediately.",
    shellGuidance(platform, options.shell),
  ].filter(Boolean).join(" ");
}

function localShellToolParameters(): Record<string, unknown> {
  return {
    type: "object",
    properties: {
      command: {
        type: "string",
        description: "Shell command to execute. Keep it focused and quote paths containing spaces.",
      },
      description: {
        type: "string",
        description: "Short human-readable description of why this command is being run.",
      },
      workdir: {
        type: "string",
        description: "Optional relative working directory. Use this instead of cd. Absolute paths are rejected by the default host runner.",
      },
      timeout_ms: {
        type: "integer",
        description: "Optional timeout in milliseconds.",
      },
    },
    required: ["command"],
    additionalProperties: false,
  };
}

function shellRequest(args: Record<string, unknown>): LocalShellRequest {
  return {
    command: requiredString(args.command, "command"),
    description: optionalString(args.description, "description"),
    workdir: optionalString(args.workdir, "workdir"),
    timeoutMs: optionalNumber(args.timeoutMs ?? args.timeout_ms, "timeout_ms"),
  };
}

function shellApprovalRequired(request: LocalShellRequest, isolationStatus: LocalShellIsolationStatus): Record<string, unknown> {
  return {
    ok: false,
    requires_approval: true,
    action: "run",
    command: request.command,
    description: request.description,
    workdir: request.workdir,
    timeout_ms: request.timeoutMs,
    shell_isolation: isolationStatus,
    message: "local_shell command execution requires approval",
  };
}

function directIsolationStatus(fallback: boolean, options: LocalShellIsolationOptions = {}): LocalShellIsolationStatus {
  const requested = normalizeIsolationOptions(options);
  const warnings = directIsolationWarnings(fallback, requested);
  return {
    executor: "direct",
    driver: "direct",
    isolated: false,
    fallback,
    requested,
    guarantees: {
      filesystem: "none",
      network: "allowed",
      user: "host-user",
      process: "host-process-tree",
      resources: "timeout-only",
    },
    warnings,
  };
}

function normalizeIsolationOptions(options: LocalShellIsolationOptions = {}): Required<LocalShellIsolationOptions> & { resources: LocalShellIsolationResourceOptions } {
  return {
    filesystem: options.filesystem ?? "host",
    network: options.network ?? "allowed",
    env: options.env ?? "inherit",
    resources: {
      memoryMb: options.resources?.memoryMb,
      cpuCount: options.resources?.cpuCount,
    },
  };
}

function directIsolationWarnings(fallback: boolean, requested: Required<LocalShellIsolationOptions> & { resources: LocalShellIsolationResourceOptions }): string[] {
  const warnings = [
    fallback
      ? "No local shell isolator is configured; falling back to direct host execution."
      : "Direct host execution has no OS-level isolation.",
  ];
  if (requested.filesystem !== "host") {
    warnings.push(`Requested filesystem isolation (${requested.filesystem}) is not enforced by direct execution.`);
  }
  if (requested.network === "blocked") {
    warnings.push("Requested network blocking is not enforced by direct execution.");
  }
  if (requested.env === "minimal") {
    warnings.push("Requested minimal environment is not enforced by direct execution.");
  }
  if (requested.resources.memoryMb != null || requested.resources.cpuCount != null) {
    warnings.push("Requested CPU or memory limits are not enforced by direct execution.");
  }
  return warnings;
}

function isolationRequestDescription(options: Required<LocalShellIsolationOptions> & { resources: LocalShellIsolationResourceOptions }): string {
  const resources = [
    options.resources.memoryMb == null ? "" : `memory_mb=${options.resources.memoryMb}`,
    options.resources.cpuCount == null ? "" : `cpu_count=${options.resources.cpuCount}`,
  ].filter(Boolean).join("; ");
  return `Requested isolation: filesystem=${options.filesystem}; network=${options.network}; env=${options.env}${resources ? `; ${resources}` : ""}.`;
}

function withIsolationStatus<T extends LocalCommandRunner>(runner: T, status: LocalShellIsolationStatus): T {
  Object.defineProperty(runner, "isolationStatus", {
    configurable: true,
    enumerable: true,
    value: status,
  });
  return runner;
}

function isolatorStatusSync(executablePath: string, driver: string): LocalShellIsolatorStatusResult {
  const request = JSON.stringify({ id: "status", method: "status", params: {} });
  const result = spawnSync(executablePath, ["--once", `--driver=${driver}`], {
    input: request,
    encoding: "utf8",
    maxBuffer: 1024 * 1024,
    windowsHide: true,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error((result.stderr || result.stdout || `agent-isolator exited with status ${result.status}`).trim());
  }
  const envelope = parseIsolatorEnvelope(result.stdout);
  if (envelope.error) {
    throw new Error(envelope.error.message || envelope.error.code || "agent-isolator status failed");
  }
  return isolatorStatusResult(envelope.result);
}

async function isolatorRequest(
  executablePath: string,
  driver: string,
  envelope: Record<string, unknown>,
  context: LocalShellContext,
): Promise<{ result: unknown }> {
  return await new Promise((resolve, reject) => {
    const child = spawn(executablePath, ["--once", `--driver=${driver}`], {
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });
    const stdout = new BoundedText(1024 * 1024);
    const stderr = new BoundedText(1024 * 1024);
    let settled = false;

    const finish = (error?: Error) => {
      if (settled) return;
      settled = true;
      context.signal?.removeEventListener("abort", abort);
      if (error) {
        reject(error);
        return;
      }
      try {
        const parsed = parseIsolatorEnvelope(stdout.text());
        if (parsed.error) {
          reject(new Error(parsed.error.message || parsed.error.code || "agent-isolator request failed"));
          return;
        }
        resolve({ result: parsed.result });
      } catch (parseError) {
        const details = stderr.text();
        reject(new Error(`${parseError instanceof Error ? parseError.message : String(parseError)}${details ? `: ${details}` : ""}`));
      }
    };

    const abort = () => {
      if (!child.killed) child.kill(process.platform === "win32" ? undefined : "SIGTERM");
    };

    if (context.signal?.aborted) {
      abort();
    } else {
      context.signal?.addEventListener("abort", abort, { once: true });
    }

    child.stdout?.on("data", (chunk: Buffer) => stdout.append(chunk));
    child.stderr?.on("data", (chunk: Buffer) => stderr.append(chunk));
    child.on("error", finish);
    child.on("close", (code) => {
      if (code !== 0) {
        finish(new Error((stderr.text() || stdout.text() || `agent-isolator exited with status ${code}`).trim()));
        return;
      }
      finish();
    });
    child.stdin?.end(JSON.stringify(envelope));
  });
}

function parseIsolatorEnvelope(text: string): { result?: unknown; error?: { code?: string; message?: string } } {
  const trimmed = text.trim();
  if (!trimmed) {
    throw new Error("agent-isolator returned an empty response");
  }
  return JSON.parse(trimmed);
}

function isolatorStatusResult(value: unknown): LocalShellIsolatorStatusResult {
  if (!isRecord(value)) {
    throw new Error("agent-isolator status result must be an object");
  }
  return {
    version: typeof value.version === "string" ? value.version : undefined,
    driver: requiredString(value.driver, "driver"),
    status: isolationStatusFromUnknown(value.status),
    drivers: Array.isArray(value.drivers)
      ? value.drivers.filter(isRecord).map((item) => ({
        name: String(item.name ?? ""),
        platform: String(item.platform ?? ""),
        available: item.available === true,
        warnings: Array.isArray(item.warnings) ? item.warnings.map(String) : undefined,
      }))
      : undefined,
  };
}

function isolatorRunResult(value: unknown): LocalShellResult {
  if (!isRecord(value)) {
    throw new Error("agent-isolator run result must be an object");
  }
  return {
    ok: value.ok === true,
    command: requiredString(value.command, "command"),
    description: optionalString(value.description, "description"),
    cwd: requiredString(value.cwd, "cwd"),
    exit_code: typeof value.exit_code === "number" ? value.exit_code : null,
    signal: typeof value.signal === "string" ? value.signal as NodeJS.Signals : null,
    stdout: typeof value.stdout === "string" ? value.stdout : "",
    stderr: typeof value.stderr === "string" ? value.stderr : "",
    output: typeof value.output === "string" ? value.output : "(no output)",
    duration_ms: typeof value.duration_ms === "number" ? value.duration_ms : 0,
    timed_out: value.timed_out === true,
    truncated: value.truncated === true,
    shell_isolation: isolationStatusFromUnknown(value.shell_isolation),
  };
}

function isolationStatusFromUnknown(value: unknown): LocalShellIsolationStatus {
  if (!isRecord(value)) {
    throw new Error("shell isolation status must be an object");
  }
  return {
    executor: value.executor === "isolator" ? "isolator" : "direct",
    driver: requiredString(value.driver, "shell_isolation.driver"),
    isolated: value.isolated === true,
    fallback: value.fallback === true,
    requested: normalizeIsolationOptions(isRecord(value.requested) ? isolationOptionsFromUnknown(value.requested) : {}),
    guarantees: isolationGuaranteesFromUnknown(value.guarantees),
    warnings: Array.isArray(value.warnings) ? value.warnings.map(String) : [],
  };
}

function isolationOptionsFromUnknown(value: Record<string, unknown>): LocalShellIsolationOptions {
  const resources = isRecord(value.resources) ? value.resources : {};
  return {
    filesystem: value.filesystem === "workdir-readonly" || value.filesystem === "workdir-readwrite" || value.filesystem === "host"
      ? value.filesystem
      : undefined,
    network: value.network === "blocked" || value.network === "allowed" ? value.network : undefined,
    env: value.env === "minimal" || value.env === "inherit" ? value.env : undefined,
    resources: {
      memoryMb: typeof resources.memoryMb === "number" ? resources.memoryMb : undefined,
      cpuCount: typeof resources.cpuCount === "number" ? resources.cpuCount : undefined,
    },
  };
}

function isolationGuaranteesFromUnknown(value: unknown): LocalShellIsolationGuarantees {
  const record = isRecord(value) ? value : {};
  return {
    filesystem: record.filesystem === "workdir-mounted" || record.filesystem === "policy-enforced" ? record.filesystem : "none",
    network: record.network === "blocked" || record.network === "configurable" ? record.network : "allowed",
    user: record.user === "unprivileged-user" || record.user === "namespace-user" ? record.user : "host-user",
    process: record.process === "child-contained" || record.process === "pid-namespace" ? record.process : "host-process-tree",
    resources: record.resources === "cpu-memory-limits" ? "cpu-memory-limits" : record.resources === "timeout-only" ? "timeout-only" : "none",
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function shellToolResult(result: LocalShellResult): Record<string, unknown> {
  return {
    ...result,
    action: "run",
  };
}

function resolveContainedPath(root: string, child?: string): string {
  if (!child) return root;
  if (path.isAbsolute(child)) {
    throw new Error("workdir must be relative to the configured local shell cwd");
  }
  const resolved = path.resolve(root, child);
  const relative = path.relative(root, resolved);
  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error("workdir must stay inside the configured local shell cwd");
  }
  return resolved;
}

function requiredString(value: unknown, name: string): string {
  if (typeof value !== "string" || !value.trim()) {
    throw new Error(`${name} must be a non-empty string`);
  }
  return value;
}

function optionalString(value: unknown, name: string): string | undefined {
  if (value == null || value === "") return undefined;
  if (typeof value !== "string") {
    throw new Error(`${name} must be a string`);
  }
  return value;
}

function optionalNumber(value: unknown, name: string): number | undefined {
  if (value == null) return undefined;
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${name} must be a number`);
  }
  return Math.trunc(value);
}

function positiveInteger(value: unknown, fallback: number, name: string): number {
  if (value == null) return fallback;
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    throw new Error(`${name} must be a positive number`);
  }
  return Math.trunc(value);
}

function shellDisplayName(shell: string | boolean | undefined): string {
  if (typeof shell === "string" && shell.trim()) return shell;
  if (shell === false) return "direct process execution";
  if (process.platform === "win32") return "cmd.exe";
  return "/bin/sh";
}

function shellGuidance(platform: NodeJS.Platform, shell: string | boolean | undefined): string {
  const name = typeof shell === "string" ? path.basename(shell).toLowerCase() : shellDisplayName(shell).toLowerCase();
  if (platform === "win32" || name.includes("powershell") || name === "pwsh") {
    return "On Windows/PowerShell, prefer PowerShell-compatible commands and quote paths with spaces.";
  }
  if (name.includes("cmd.exe") || name === "cmd") {
    return "On cmd.exe, prefer cmd-compatible syntax and use && only for dependent command chaining.";
  }
  return "On POSIX shells, prefer portable shell syntax, quote paths with spaces, and use && only when later commands depend on earlier success.";
}

class BoundedText {
  private chunks: Buffer[] = [];
  private size = 0;
  truncated = false;

  constructor(private readonly maxBytes: number) {}

  append(chunk: Buffer): void {
    this.chunks.push(chunk);
    this.size += chunk.byteLength;
    while (this.size > this.maxBytes && this.chunks.length > 1) {
      const removed = this.chunks.shift();
      if (!removed) break;
      this.size -= removed.byteLength;
      this.truncated = true;
    }
    if (this.size > this.maxBytes && this.chunks.length === 1) {
      const current = this.chunks[0];
      this.chunks[0] = current.subarray(current.byteLength - this.maxBytes);
      this.size = this.chunks[0].byteLength;
      this.truncated = true;
    }
  }

  text(): string {
    const value = Buffer.concat(this.chunks, this.size).toString("utf8");
    return this.truncated ? `...output truncated...\n${value}` : value;
  }
}
