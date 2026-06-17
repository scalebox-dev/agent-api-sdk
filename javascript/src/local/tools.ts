import type { FunctionTool } from "../types/tools.js";
import { createLocalContextPackage, type LocalContextManifest } from "./context.js";
import type {
  LocalGrepResponse,
  LocalReadFileParams,
  LocalSummary,
  LocalWorkspace,
  LocalWorkspaceEditPlan,
  LocalWorkspaceEditResult,
  LocalWorkspaceLineEdit,
} from "./core.js";

export type LocalWorkspaceAccessMode = "approval" | "full";

export type LocalWorkspaceAction =
  | "summarize"
  | "list"
  | "search"
  | "grep"
  | "read"
  | "read_lines"
  | "context"
  | "snapshot"
  | "classify_path"
  | "preview_edits"
  | "apply_edits"
  | "write"
  | "mkdir"
  | "delete";

export interface LocalWorkspaceToolRegistryOptions {
  accessMode?: LocalWorkspaceAccessMode;
  toolName?: string;
}

export interface LocalWorkspaceToolRegistry {
  readonly workspace: LocalWorkspace;
  readonly accessMode: LocalWorkspaceAccessMode;
  readonly toolName: string;
  definitions(): FunctionTool[];
  handlers(): Record<string, (args: Record<string, unknown>) => Promise<Record<string, unknown>>>;
  execute(name: string, args: Record<string, unknown>): Promise<Record<string, unknown>>;
  requiresApproval(name: string, args?: Record<string, unknown>): boolean;
}

export interface LocalWorkspaceDriverOptions {
  accessMode?: LocalWorkspaceAccessMode;
}

export class LocalWorkspaceDriver {
  readonly workspace: LocalWorkspace;
  readonly accessMode: LocalWorkspaceAccessMode;

  constructor(workspace: LocalWorkspace, options: LocalWorkspaceDriverOptions = {}) {
    this.workspace = workspace;
    this.accessMode = options.accessMode ?? "approval";
  }

  async dispatch(args: Record<string, unknown>): Promise<Record<string, unknown>> {
    const action = workspaceAction(args);
    switch (action) {
      case "summarize":
        return localToolResult(action, localSummaryResult(await this.workspace.summarize(summaryArgs(args))));
      case "list":
        return localToolResult(action, await this.workspace.listEntries(optionalStringArg(args, "path") ?? ".", listArgs(args)));
      case "search":
        return localToolResult(action, await this.workspace.searchEntries(searchEntriesArgs(args)));
      case "grep":
        return localToolResult(action, await this.workspace.grep(grepArgs(args)) satisfies LocalGrepResponse);
      case "read":
        return localToolResult(action, await this.workspace.readFile(stringArg(args, "path"), readFileArgs(args)));
      case "read_lines":
        return localToolResult(action, await this.workspace.readLines(stringArg(args, "path"), readLinesArgs(args)));
      case "context":
        return localToolResult(action, await createLocalContextPackage(this.workspace, contextPackageArgs(args)) satisfies LocalContextManifest);
      case "snapshot":
        return localToolResult(action, await this.workspace.snapshot(snapshotArgs(args)));
      case "classify_path":
        return localToolResult(action, this.workspace.classifyPath(stringArg(args, "path")));
      case "preview_edits":
        return localToolResult(action, await this.workspace.previewEdits(editsArg(args)) satisfies LocalWorkspaceEditPlan);
      case "apply_edits":
        return await this.dispatchApplyEdits(args);
      case "write":
        return await this.dispatchWrite(args);
      case "mkdir":
        return await this.dispatchMkdir(args);
      case "delete":
        return await this.dispatchDelete(args);
      default:
        throw new Error(`unsupported local_workspace action: ${action satisfies never}`);
    }
  }

  requiresApproval(args: Record<string, unknown>): boolean {
    if (this.accessMode === "full") {
      return false;
    }
    return mutatingLocalWorkspaceActions.has(workspaceAction(args));
  }

  private async dispatchApplyEdits(args: Record<string, unknown>): Promise<Record<string, unknown>> {
    const edits = editsArg(args);
    if (this.accessMode !== "full") {
      return approvalRequired("apply_edits", args, await this.workspace.previewEdits(edits));
    }
    return localToolResult("apply_edits", await this.workspace.applyEdits(edits) satisfies LocalWorkspaceEditResult);
  }

  private async dispatchWrite(args: Record<string, unknown>): Promise<Record<string, unknown>> {
    if (this.accessMode !== "full") {
      return approvalRequired("write", args);
    }
    return localToolResult("write", await this.workspace.writeText(stringArg(args, "path"), stringArg(args, "content")));
  }

  private async dispatchMkdir(args: Record<string, unknown>): Promise<Record<string, unknown>> {
    if (this.accessMode !== "full") {
      return approvalRequired("mkdir", args);
    }
    return localToolResult("mkdir", await this.workspace.createDirectory(stringArg(args, "path")));
  }

  private async dispatchDelete(args: Record<string, unknown>): Promise<Record<string, unknown>> {
    if (this.accessMode !== "full") {
      return approvalRequired("delete", args);
    }
    return localToolResult("delete", await this.workspace.deletePath(stringArg(args, "path")));
  }
}

export function createLocalWorkspaceToolRegistry(
  workspace: LocalWorkspace,
  options: LocalWorkspaceToolRegistryOptions = {},
): LocalWorkspaceToolRegistry {
  const toolName = options.toolName ?? "local_workspace";
  const driver = new LocalWorkspaceDriver(workspace, { accessMode: options.accessMode });
  const definition = localWorkspaceToolDefinition(toolName);

  return {
    workspace,
    accessMode: driver.accessMode,
    toolName,
    definitions: () => [{ ...definition }],
    handlers: () => ({ [toolName]: (args: Record<string, unknown>) => driver.dispatch(args) }),
    execute: async (name: string, args: Record<string, unknown>) => {
      if (name !== toolName) {
        throw new Error(`unknown local workspace tool: ${name}`);
      }
      return await driver.dispatch(args);
    },
    requiresApproval: (name: string, args: Record<string, unknown> = {}) => name === toolName && driver.requiresApproval(args),
  };
}

export function localWorkspaceToolDefinition(name = "local_workspace"): FunctionTool {
  return {
    type: "function",
    name,
    description: localWorkspaceToolDescription,
    parameters: localWorkspaceToolParameters(),
    strict: false,
  };
}

export function localWorkspaceToolInstructions(): string {
  return localWorkspaceToolDescription;
}

const localWorkspaceActions: LocalWorkspaceAction[] = [
  "summarize",
  "list",
  "search",
  "grep",
  "read",
  "read_lines",
  "context",
  "snapshot",
  "classify_path",
  "preview_edits",
  "apply_edits",
  "write",
  "mkdir",
  "delete",
];

const mutatingLocalWorkspaceActions = new Set<LocalWorkspaceAction>([
  "apply_edits",
  "write",
  "mkdir",
  "delete",
]);

const localWorkspaceToolDescription = [
  "Inspect and modify the selected local workspace through one model-facing primitive.",
  "Use action=list/search/grep/summarize/context to discover files, read/read_lines for file content, preview_edits before edits, and apply_edits/write/mkdir/delete only when mutation is intended.",
  "In approval mode, mutating actions return requires_approval with a safe preview instead of changing files. In full mode, mutating actions execute immediately.",
  "Paths are relative to the selected local workspace; never use absolute paths.",
].join(" ");

function localWorkspaceToolParameters(): Record<string, unknown> {
  return objectSchema({
    action: {
      type: "string",
      enum: localWorkspaceActions,
      description: "Workspace operation. Prefer summarize/list/search/grep before reading or editing. Prefer read_lines and apply_edits for source changes.",
    },
    path: stringSchema("Relative path. File path for read/write/delete/edit actions; directory base for list/search/grep/summarize/context/snapshot."),
    query: stringSchema("Path/name query for search, or optional context query."),
    pattern: stringSchema("Literal text pattern for grep."),
    content: stringSchema("Text content for write."),
    start_line: integerSchema("1-based start line for read_lines and edit entries."),
    end_line: integerSchema("1-based inclusive end line; omit or 0 for EOF when supported."),
    replacement: stringSchema("Replacement text for simple single edit flows."),
    edits: {
      type: "array",
      minItems: 1,
      description: "Line edits for preview_edits/apply_edits.",
      items: objectSchema({
        path: requiredStringSchema("Relative file path."),
        start_line: requiredIntegerSchema("1-based start line."),
        end_line: integerSchema("1-based inclusive end line."),
        replacement: stringSchema("Replacement text. Empty string deletes the line range."),
        expected_sha256: stringSchema("Optional expected SHA-256 for conflict detection."),
      }, ["path", "start_line"]),
    },
    options: objectSchema({
      recursive: booleanSchema("List recursively."),
      include_directories: booleanSchema("Include directories in list results."),
      max_depth: integerSchema("Maximum recursive list depth."),
      limit: integerSchema("Maximum entries or matches."),
      max_files: integerSchema("Maximum files to scan or package."),
      max_bytes: integerSchema("Maximum total bytes to read/package."),
      max_bytes_per_file: integerSchema("Maximum bytes per file."),
      max_previews: integerSchema("Maximum summary previews."),
      include_content: booleanSchema("Include file contents in context packages."),
      include_summary: booleanSchema("Include workspace summary in context packages."),
      include_search: booleanSchema("Include grep results in context packages when query is set."),
      include_secrets: booleanSchema("Include likely secret file contents in context packages."),
      hash: booleanSchema("Include SHA-256 hashes in snapshots."),
    }),
  }, ["action"]);
}

function workspaceAction(args: Record<string, unknown>): LocalWorkspaceAction {
  const value = stringArg(args, "action").trim().toLowerCase();
  if (!localWorkspaceActions.includes(value as LocalWorkspaceAction)) {
    throw new Error(`unsupported local_workspace action: ${value}`);
  }
  return value as LocalWorkspaceAction;
}

function summaryArgs(args: Record<string, unknown>) {
  return {
    path: optionalStringArg(args, "path"),
    maxFiles: optionalNumberArg(args, "maxFiles", "max_files"),
    maxPreviews: optionalNumberArg(args, "maxPreviews", "max_previews"),
  };
}

function grepArgs(args: Record<string, unknown>) {
  return {
    pattern: stringArg(args, "pattern"),
    path: optionalStringArg(args, "path"),
    limit: optionalNumberArg(args, "limit"),
    maxFiles: optionalNumberArg(args, "maxFiles", "max_files"),
  };
}

function listArgs(args: Record<string, unknown>) {
  return {
    recursive: optionalBooleanArg(args, "recursive"),
    includeDirectories: optionalBooleanArg(args, "includeDirectories", "include_directories"),
    maxDepth: optionalNumberArg(args, "maxDepth", "max_depth"),
  };
}

function searchEntriesArgs(args: Record<string, unknown>) {
  return {
    query: stringArg(args, "query"),
    path: optionalStringArg(args, "path"),
    limit: optionalNumberArg(args, "limit"),
  };
}

function readFileArgs(args: Record<string, unknown>): LocalReadFileParams {
  return {
    maxBytes: optionalNumberArg(args, "maxBytes", "max_bytes"),
  };
}

function readLinesArgs(args: Record<string, unknown>) {
  return {
    startLine: numberArg(args, "startLine", "start_line"),
    endLine: optionalNumberArg(args, "endLine", "end_line"),
    maxBytes: optionalNumberArg(args, "maxBytes", "max_bytes"),
  };
}

function snapshotArgs(args: Record<string, unknown>) {
  return {
    path: optionalStringArg(args, "path"),
    hash: optionalBooleanArg(args, "hash"),
    maxBytesPerFile: optionalNumberArg(args, "maxBytesPerFile", "max_bytes_per_file"),
  };
}

function contextPackageArgs(args: Record<string, unknown>) {
  return {
    path: optionalStringArg(args, "path"),
    query: optionalStringArg(args, "query"),
    maxFiles: optionalNumberArg(args, "maxFiles", "max_files"),
    maxBytes: optionalNumberArg(args, "maxBytes", "max_bytes"),
    maxBytesPerFile: optionalNumberArg(args, "maxBytesPerFile", "max_bytes_per_file"),
    includeContent: optionalBooleanArg(args, "includeContent", "include_content"),
    includeSummary: optionalBooleanArg(args, "includeSummary", "include_summary"),
    includeSearch: optionalBooleanArg(args, "includeSearch", "include_search"),
    includeSecrets: optionalBooleanArg(args, "includeSecrets", "include_secrets"),
  };
}

function editsArg(args: Record<string, unknown>): LocalWorkspaceLineEdit[] {
  const edits = args.edits;
  if (Array.isArray(edits) && edits.length > 0) {
    return edits.map((edit) => {
      if (!edit || typeof edit !== "object") {
        throw new Error("each edit must be an object");
      }
      return editArg(edit as Record<string, unknown>);
    });
  }
  if (typeof args.path === "string" && typeof (args.startLine ?? args.start_line) === "number") {
    return [editArg(args)];
  }
  throw new Error("edits must be a non-empty array");
}

function editArg(record: Record<string, unknown>): LocalWorkspaceLineEdit {
  return {
    path: stringArg(record, "path"),
    startLine: numberArg(record, "startLine", "start_line"),
    endLine: optionalNumberArg(record, "endLine", "end_line"),
    replacement: typeof record.replacement === "string" ? record.replacement : "",
    expectedSha256: optionalStringArg(record, "expectedSha256", "expected_sha256"),
  };
}

function localSummaryResult(summary: LocalSummary): Record<string, unknown> {
  return summary as unknown as Record<string, unknown>;
}

function localToolResult(action: LocalWorkspaceAction, value: unknown): Record<string, unknown> {
  const result = value as Record<string, unknown>;
  if (result && typeof result === "object" && !Array.isArray(result)) {
    return { ok: true, action, ...result };
  }
  return { ok: true, action, result };
}

function stringArg(args: Record<string, unknown>, key: string, alternateKey?: string): string {
  const direct = args[key] ?? (alternateKey ? args[alternateKey] : undefined);
  const fromOptions = args.options && typeof args.options === "object"
    ? (args.options as Record<string, unknown>)[key] ?? (alternateKey ? (args.options as Record<string, unknown>)[alternateKey] : undefined)
    : undefined;
  const value = direct ?? fromOptions;
  if (typeof value !== "string" || !value.trim()) {
    throw new Error(`${key} must be a non-empty string`);
  }
  return value;
}

function optionalStringArg(args: Record<string, unknown>, key: string, alternateKey?: string): string | undefined {
  const direct = args[key] ?? (alternateKey ? args[alternateKey] : undefined);
  const fromOptions = args.options && typeof args.options === "object"
    ? (args.options as Record<string, unknown>)[key] ?? (alternateKey ? (args.options as Record<string, unknown>)[alternateKey] : undefined)
    : undefined;
  const value = direct ?? fromOptions;
  if (value == null || value === "") return undefined;
  if (typeof value !== "string") {
    throw new Error(`${key} must be a string`);
  }
  return value;
}

function numberArg(args: Record<string, unknown>, key: string, alternateKey?: string): number {
  const direct = args[key] ?? (alternateKey ? args[alternateKey] : undefined);
  const fromOptions = args.options && typeof args.options === "object"
    ? (args.options as Record<string, unknown>)[key] ?? (alternateKey ? (args.options as Record<string, unknown>)[alternateKey] : undefined)
    : undefined;
  const value = direct ?? fromOptions;
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${key} must be a number`);
  }
  return Math.trunc(value);
}

function optionalNumberArg(args: Record<string, unknown>, key: string, alternateKey?: string): number | undefined {
  const direct = args[key] ?? (alternateKey ? args[alternateKey] : undefined);
  const fromOptions = args.options && typeof args.options === "object"
    ? (args.options as Record<string, unknown>)[key] ?? (alternateKey ? (args.options as Record<string, unknown>)[alternateKey] : undefined)
    : undefined;
  const value = direct ?? fromOptions;
  if (value == null) return undefined;
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${key} must be a number`);
  }
  return Math.trunc(value);
}

function optionalBooleanArg(args: Record<string, unknown>, key: string, alternateKey?: string): boolean | undefined {
  const direct = args[key] ?? (alternateKey ? args[alternateKey] : undefined);
  const fromOptions = args.options && typeof args.options === "object"
    ? (args.options as Record<string, unknown>)[key] ?? (alternateKey ? (args.options as Record<string, unknown>)[alternateKey] : undefined)
    : undefined;
  const value = direct ?? fromOptions;
  if (value == null) return undefined;
  if (typeof value !== "boolean") {
    throw new Error(`${key} must be a boolean`);
  }
  return value;
}

function approvalRequired(action: LocalWorkspaceAction, args: Record<string, unknown>, preview?: unknown): Record<string, unknown> {
  return {
    ok: false,
    action,
    requires_approval: true,
    arguments: args,
    preview,
    message: `local_workspace action ${action} requires approval`,
  };
}

function objectSchema(properties: Record<string, unknown>, required: string[] = []): Record<string, unknown> {
  return {
    type: "object",
    properties,
    required,
    additionalProperties: false,
  };
}

function stringSchema(description: string): Record<string, unknown> {
  return { type: "string", description };
}

function requiredStringSchema(description: string): Record<string, unknown> {
  return stringSchema(description);
}

function integerSchema(description: string): Record<string, unknown> {
  return { type: "integer", description };
}

function booleanSchema(description: string): Record<string, unknown> {
  return { type: "boolean", description };
}

function requiredIntegerSchema(description: string): Record<string, unknown> {
  return integerSchema(description);
}
