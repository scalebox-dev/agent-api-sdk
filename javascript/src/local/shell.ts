import { spawn } from "node:child_process";
import path from "node:path";

import type { FunctionTool } from "../types/tools.js";
import type { LocalWorkdir } from "./core.js";

export type LocalShellAccessMode = "approval" | "full";

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
}

export interface LocalCommandRunner {
  run(request: LocalShellRequest, context?: LocalShellContext): Promise<LocalShellResult>;
}

export interface HostLocalShellRunnerOptions {
  cwd?: string;
  shell?: string | boolean;
  timeoutMs?: number;
  maxOutputBytes?: number;
  env?: NodeJS.ProcessEnv;
}

export class HostLocalShellRunner implements LocalCommandRunner {
  readonly cwd: string;
  readonly shell: string | boolean;
  readonly timeoutMs: number;
  readonly maxOutputBytes: number;
  readonly env: NodeJS.ProcessEnv;

  constructor(options: HostLocalShellRunnerOptions = {}) {
    this.cwd = path.resolve(options.cwd ?? process.cwd());
    this.shell = options.shell ?? true;
    this.timeoutMs = positiveInteger(options.timeoutMs, 2 * 60 * 1000, "timeoutMs");
    this.maxOutputBytes = positiveInteger(options.maxOutputBytes, 128 * 1024, "maxOutputBytes");
    this.env = { ...process.env, ...(options.env ?? {}) };
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

export interface LocalShellToolRegistryOptions {
  accessMode?: LocalShellAccessMode;
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

  constructor(options: LocalShellToolRegistryOptions = {}) {
    const cwd = options.workdir?.root ?? options.cwd;
    this.accessMode = options.accessMode ?? "approval";
    this.runner = options.runner ?? new HostLocalShellRunner({
      cwd,
      shell: options.shell,
      timeoutMs: options.timeoutMs,
      maxOutputBytes: options.maxOutputBytes,
    });
  }

  async dispatch(args: Record<string, unknown>, context: LocalShellContext = {}): Promise<Record<string, unknown>> {
    const request = shellRequest(args);
    if (this.accessMode !== "full") {
      return shellApprovalRequired(request);
    }
    return shellToolResult(await this.runner.run(request, context));
  }

  requiresApproval(): boolean {
    return this.accessMode !== "full";
  }
}

export function createLocalShellToolRegistry(options: LocalShellToolRegistryOptions = {}): LocalShellToolRegistry {
  const toolName = options.toolName ?? "local_shell";
  const driver = new LocalShellDriver(options);
  const hostRunner = driver.runner instanceof HostLocalShellRunner ? driver.runner : undefined;
  const definition = localShellToolDefinition(toolName, {
    accessMode: driver.accessMode,
    cwd: options.workdir?.root ?? hostRunner?.cwd ?? options.cwd,
    shell: hostRunner?.shell ?? options.shell,
    timeoutMs: hostRunner?.timeoutMs ?? options.timeoutMs,
    maxOutputBytes: hostRunner?.maxOutputBytes ?? options.maxOutputBytes,
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
  const cwd = options.cwd ? path.resolve(options.cwd) : undefined;
  const timeoutMs = options.timeoutMs ?? 2 * 60 * 1000;
  const maxOutputBytes = options.maxOutputBytes ?? 128 * 1024;
  return [
    "Run a local shell command through one model-facing primitive.",
    "Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
    `Execution environment: platform=${platform}; shell=${shell}; access_mode=${accessMode}; default_timeout_ms=${timeoutMs}; max_output_bytes=${maxOutputBytes}.`,
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

function shellApprovalRequired(request: LocalShellRequest): Record<string, unknown> {
  return {
    ok: false,
    requires_approval: true,
    action: "run",
    command: request.command,
    description: request.description,
    workdir: request.workdir,
    timeout_ms: request.timeoutMs,
    message: "local_shell command execution requires approval",
  };
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
