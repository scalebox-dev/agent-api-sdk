import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";

import {
  classifyLocalPathSensitivity,
  type LocalFileDeliver,
  type LocalGrepResponse,
  type LocalPathSensitivity,
  type LocalPathSensitivityInfo,
  type LocalSummary,
  type LocalWorkspace,
  type LocalWorkspaceSnapshotFile,
} from "./core.js";

export interface LocalContextPackageParams {
  path?: string;
  include?: string[];
  exclude?: string[];
  query?: string;
  maxFiles?: number;
  maxBytes?: number;
  maxBytesPerFile?: number;
  previewBytes?: number;
  includeContent?: boolean;
  includeHashes?: boolean;
  includeSummary?: boolean;
  includeSearch?: boolean;
  includeSecrets?: boolean;
}

export interface LocalContextFile {
  path: string;
  size: number;
  sha256?: string;
  mime_type?: string;
  sensitivity: LocalPathSensitivity;
  sensitivity_reason?: string;
  content?: string;
  content_base64?: string;
  encoding?: LocalFileDeliver["encoding"];
  truncated?: boolean;
  omitted_reason?: string;
}

export interface LocalContextManifest {
  object: "local_context_manifest";
  root: string;
  workspace_name: string;
  generated_at_unix: number;
  base_path: string;
  file_count: number;
  total_bytes: number;
  included_bytes: number;
  truncated: boolean;
  files: LocalContextFile[];
  summary?: LocalSummary;
  search?: LocalGrepResponse;
}

export async function createLocalContextPackage(
  workspace: LocalWorkspace,
  params: LocalContextPackageParams = {},
): Promise<LocalContextManifest> {
  const basePath = params.path ?? ".";
  const maxFiles = positiveInt(params.maxFiles, 80);
  const maxBytes = positiveInt(params.maxBytes, 256 * 1024);
  const maxBytesPerFile = positiveInt(params.maxBytesPerFile, 32 * 1024);
  const previewBytes = positiveInt(params.previewBytes, maxBytesPerFile);
  const includeContent = params.includeContent ?? true;
  const includeHashes = params.includeHashes ?? true;
  const includeSummary = params.includeSummary ?? true;
  const includeSearch = Boolean(params.includeSearch && params.query?.trim());
  const includeSecrets = params.includeSecrets ?? false;

  const stats = await workspace.list(basePath, { recursive: true });
  const fileStats = stats
    .filter((item) => item.type === "file")
    .filter((item) => matchesPathFilters(item.path, params.include, params.exclude))
    .sort((a, b) => a.path.localeCompare(b.path));

  const files: LocalContextFile[] = [];
  let includedBytes = 0;
  let truncated = fileStats.length > maxFiles;

  for (const item of fileStats.slice(0, maxFiles)) {
    const sensitivity = classifyLocalPathSensitivity(item.path);
    const baseFile: LocalContextFile = {
      path: item.path,
      size: item.size,
      sensitivity: sensitivity.sensitivity,
      sensitivity_reason: sensitivity.reason,
    };

    if (!includeSecrets && sensitivity.sensitivity === "secret") {
      files.push({ ...baseFile, omitted_reason: "secret_path" });
      continue;
    }
    if (item.size > maxBytesPerFile) {
      files.push({ ...baseFile, omitted_reason: "file_too_large" });
      truncated = true;
      continue;
    }
    if (includedBytes >= maxBytes) {
      files.push({ ...baseFile, omitted_reason: "package_budget_exceeded" });
      truncated = true;
      continue;
    }

    const remaining = maxBytes - includedBytes;
    const readBudget = Math.min(previewBytes, maxBytesPerFile, remaining);
    if (readBudget <= 0) {
      files.push({ ...baseFile, omitted_reason: "package_budget_exceeded" });
      truncated = true;
      continue;
    }

    const delivered = await workspace.readFile(item.path, { maxBytes: readBudget });
    includedBytes += delivered.encoding === "text"
      ? Buffer.byteLength(delivered.content ?? "", "utf8")
      : Buffer.byteLength(delivered.content_base64 ?? "", "base64");

    const packaged: LocalContextFile = {
      ...baseFile,
      mime_type: delivered.mime_type,
      encoding: delivered.encoding,
      truncated: delivered.truncated || undefined,
    };
    if (includeContent) {
      packaged.content = delivered.content;
      packaged.content_base64 = delivered.content_base64;
    }
    if (includeHashes) {
      packaged.sha256 = await sha256File(workspace, item.path);
    }
    if (delivered.truncated) {
      truncated = true;
    }
    files.push(packaged);
  }

  const summary = includeSummary
    ? await workspace.summarize({
        path: basePath,
        maxFiles,
        previewBytes,
        ignore: params.exclude,
      })
    : undefined;
  const search = includeSearch
    ? await workspace.grep({
        pattern: params.query!,
        path: basePath,
        limit: maxFiles,
        maxBytesPerFile,
        ignore: params.exclude,
      })
    : undefined;

  return {
    object: "local_context_manifest",
    root: workspace.root,
    workspace_name: workspace.name,
    generated_at_unix: Math.floor(Date.now() / 1000),
    base_path: basePath,
    file_count: fileStats.length,
    total_bytes: fileStats.reduce((sum, item) => sum + item.size, 0),
    included_bytes: includedBytes,
    truncated,
    files,
    summary,
    search,
  };
}

export function summarizeLocalContextSensitivity(files: Array<Pick<LocalContextFile, "path" | "sensitivity" | "sensitivity_reason">>): {
  highest: LocalPathSensitivity;
  files: LocalPathSensitivityInfo[];
} {
  const order: Record<LocalPathSensitivity, number> = { normal: 0, sensitive: 1, secret: 2 };
  let highest: LocalPathSensitivity = "normal";
  const sensitiveFiles: LocalPathSensitivityInfo[] = [];
  for (const file of files) {
    if (order[file.sensitivity] > order[highest]) {
      highest = file.sensitivity;
    }
    if (file.sensitivity !== "normal") {
      sensitiveFiles.push({
        path: file.path,
        sensitivity: file.sensitivity,
        reason: file.sensitivity_reason,
      });
    }
  }
  return { highest, files: sensitiveFiles };
}

function matchesPathFilters(relativePath: string, include?: string[], exclude?: string[]): boolean {
  if (include?.length && !include.some((pattern) => pathMatches(relativePath, pattern))) {
    return false;
  }
  if (exclude?.some((pattern) => pathMatches(relativePath, pattern))) {
    return false;
  }
  return true;
}

function pathMatches(relativePath: string, pattern: string): boolean {
  const clean = pattern.replace(/\\/g, "/").replace(/^\/+/, "");
  if (!clean || clean === ".") {
    return true;
  }
  if (!clean.includes("*")) {
    return relativePath === clean || relativePath.startsWith(`${clean}/`);
  }
  const escaped = clean
    .split("*")
    .map((part) => part.replace(/[.+?^${}()|[\]\\]/g, "\\$&"))
    .join(".*");
  return new RegExp(`^${escaped}$`).test(relativePath);
}

function positiveInt(value: number | undefined, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

async function sha256File(workspace: LocalWorkspace, relativePath: string): Promise<string> {
  const fullPath = workspace.files.resolvePath(relativePath);
  return createHash("sha256").update(await readFile(fullPath)).digest("hex");
}

export type LocalContextSnapshotFile = LocalWorkspaceSnapshotFile;
