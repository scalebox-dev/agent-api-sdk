import { createHash, randomUUID } from "node:crypto";
import { constants as fsConstants, watch as watchFS } from "node:fs";
import { access, copyFile, lstat, mkdir, readdir, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import path from "node:path";

import { localSkillFromDirectory } from "../local-skills.js";
import type { LocalSkillDirectoryOptions } from "../local-skills.js";
import type { LocalSkillDescriptor } from "../types/skills.js";

export interface LocalAppDirs {
  home: string;
  data: string;
  config: string;
  cache: string;
  logs: string;
  temp: string;
}

export interface LocalRuntimeOptions {
  appName: string;
  appAuthor?: string;
  baseDir?: string;
  dirs?: Partial<Omit<LocalAppDirs, "home">>;
  env?: Record<string, string | undefined>;
  platform?: NodeJS.Platform;
}

export interface LocalFileStoreOptions {
  label?: string;
}

export class LocalError extends Error {
  readonly code: string;
  readonly path?: string;

  constructor(code: string, message: string, options: { path?: string; cause?: unknown } = {}) {
    super(message, { cause: options.cause });
    this.name = "LocalError";
    this.code = code;
    this.path = options.path;
  }
}

export class LocalPathError extends LocalError {
  constructor(message: string, path?: string) {
    super("local_path_error", message, { path });
    this.name = "LocalPathError";
  }
}

export class LocalIgnoredPathError extends LocalError {
  constructor(path: string) {
    super("local_ignored_path", `local workdir path is ignored: ${path}`, { path });
    this.name = "LocalIgnoredPathError";
  }
}

export class LocalFileTooLargeError extends LocalError {
  constructor(path: string) {
    super("local_file_too_large", `local file is too large: ${path}`, { path });
    this.name = "LocalFileTooLargeError";
  }
}

export class LocalNotTextFileError extends LocalError {
  constructor(path: string) {
    super("local_not_text_file", `local file must be text: ${path}`, { path });
    this.name = "LocalNotTextFileError";
  }
}

export class LocalConfigError extends LocalError {
  constructor(message: string, path?: string) {
    super("local_config_error", message, { path });
    this.name = "LocalConfigError";
  }
}

export type LocalFileType = "file" | "directory" | "symlink" | "other";

export interface LocalFileStat {
  path: string;
  fullPath: string;
  type: LocalFileType;
  size: number;
  modifiedAt: Date;
}

export interface LocalListOptions {
  recursive?: boolean;
  includeDirectories?: boolean;
  maxDepth?: number;
  ignore?: Array<string | RegExp | ((relativePath: string) => boolean)>;
}

export type LocalIgnoreRule = NonNullable<LocalListOptions["ignore"]>[number];

export interface LocalEntry {
  path: string;
  is_dir: boolean;
  size: number;
  modified_at_unix?: number;
}

export interface LocalEntryList {
  object: "list";
  entries: LocalEntry[];
}

export interface LocalSearchEntriesParams {
  query: string;
  path?: string;
  limit?: number;
}

export interface LocalReadFileParams {
  maxBytes?: number;
  format?: "text";
}

export interface LocalReadFileRawParams {
  maxBytes?: number;
  format: "raw";
}

export interface LocalFileDeliver {
  path: string;
  encoding: "text" | "base64";
  mime_type: string;
  size: number;
  truncated: boolean;
  content?: string;
  content_base64?: string;
}

export interface LocalFileRaw {
  path: string;
  size: number;
  truncated: boolean;
  content: Uint8Array;
  content_type?: string;
}

export interface LocalFileWrite {
  path: string;
  size: number;
}

export interface LocalPathDelete {
  path: string;
  recursive: boolean;
}

export interface LocalReadLinesParams {
  startLine: number;
  endLine?: number;
  maxBytes?: number;
}

export interface LocalFileLines {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  lines: string[];
  file_truncated: boolean;
  size: number;
}

export interface LocalPatchLinesParams {
  startLine: number;
  endLine?: number;
  replacement?: string;
  maxBytes?: number;
}

export interface LocalFileLinesPatch {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  size: number;
}

export interface LocalGrepParams {
  pattern: string;
  path?: string;
  limit?: number;
  maxFiles?: number;
  maxBytesPerFile?: number;
  maxLineLength?: number;
  ignore?: LocalListOptions["ignore"];
}

export interface LocalGrepMatch {
  path: string;
  line_number: number;
  line: string;
}

export interface LocalGrepResponse {
  object: "list";
  matches: LocalGrepMatch[];
  files_scanned: number;
  scan_truncated: boolean;
}

export interface LocalSummarizeParams {
  path?: string;
  maxFiles?: number;
  maxPreviews?: number;
  previewBytes?: number;
  topPaths?: number;
  ignore?: LocalListOptions["ignore"];
}

export interface LocalSummaryPreview {
  path: string;
  size: number;
  preview: string;
  preview_truncated?: boolean;
}

export interface LocalSummary {
  summary_path: string;
  file_count: number;
  total_bytes: number;
  top_paths_by_size: string[];
  text_previews: LocalSummaryPreview[];
  generated_at_unix: number;
  scan_truncated: boolean;
}

export interface LocalWorkdirOptions {
  name?: string;
  metadata?: Record<string, unknown>;
  trusted?: boolean;
  ignore?: LocalIgnoreRule[];
  gitignore?: boolean;
  maxFileBytes?: number;
}

export interface LocalWorkdirSnapshotParams {
  path?: string;
  hash?: boolean;
  maxBytesPerFile?: number;
}

export interface LocalWorkdirSnapshotFile {
  path: string;
  size: number;
  modified_at_unix?: number;
  sha256?: string;
}

export interface LocalWorkdirSnapshot {
  root: string;
  name: string;
  generated_at_unix: number;
  files: LocalWorkdirSnapshotFile[];
}

export interface LocalWorkdirDiff {
  added: LocalWorkdirSnapshotFile[];
  modified: Array<{ before: LocalWorkdirSnapshotFile; after: LocalWorkdirSnapshotFile }>;
  deleted: LocalWorkdirSnapshotFile[];
  unchanged: LocalWorkdirSnapshotFile[];
}

export interface LocalLinePatchPreview {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  before: string[];
  after: string[];
}

export interface LocalWorkdirLineEdit {
  path: string;
  startLine: number;
  endLine?: number;
  replacement?: string;
  expectedSha256?: string;
}

export interface LocalWorkdirEditPlan {
  edits: LocalWorkdirLineEdit[];
  previews: LocalLinePatchPreview[];
}

export interface LocalWorkdirEditResult {
  applied: LocalFileLinesPatch[];
  backups: Array<{ path: string; content: string }>;
}

export type LocalPathSensitivity = "normal" | "sensitive" | "secret";

export interface LocalPathSensitivityInfo {
  path: string;
  sensitivity: LocalPathSensitivity;
  reason?: string;
}

export interface LocalWorkdirWatchEvent {
  type: "change" | "rename";
  path: string;
}

export interface LocalWorkdirWatcher {
  close(): void;
}

export interface LocalSkillDiscoveryOptions extends LocalSkillDirectoryOptions {
  roots?: string[];
  recursive?: boolean;
  maxDepth?: number;
}

export interface LocalRuntime {
  appName: string;
  dirs: LocalAppDirs;
  files: LocalFileStore;
  data: LocalFileStore;
  cache: LocalFileStore;
  logs: LocalFileStore;
  temp: LocalFileStore;
  config: LocalConfigStore;
  skills: LocalSkillStore;
  workdirs: LocalWorkdirManager;
  workdir(root: string, options?: LocalWorkdirOptions): LocalWorkdir;
  ensure(): Promise<void>;
}

export function createLocalRuntime(options: LocalRuntimeOptions): LocalRuntime {
  const appName = normalizeAppName(options.appName);
  const dirs = localAppDirs({ ...options, appName });
  const data = new LocalFileStore(dirs.data, { label: "data" });
  const cache = new LocalFileStore(dirs.cache, { label: "cache" });
  const logs = new LocalFileStore(dirs.logs, { label: "logs" });
  const temp = new LocalFileStore(dirs.temp, { label: "temp" });
  const configFiles = new LocalFileStore(dirs.config, { label: "config" });
  return {
    appName,
    dirs,
    files: data,
    data,
    cache,
    logs,
    temp,
    config: new LocalConfigStore(configFiles),
    skills: new LocalSkillStore(data.child("skills")),
    workdirs: new LocalWorkdirManager(),
    workdir(root: string, workdirOptions: LocalWorkdirOptions = {}) {
      return new LocalWorkdir(root, workdirOptions);
    },
    async ensure() {
      await Promise.all([data.ensure(), cache.ensure(), logs.ensure(), temp.ensure(), configFiles.ensure()]);
    },
  };
}

export function localAppDirs(options: LocalRuntimeOptions): LocalAppDirs {
  const appName = normalizeAppName(options.appName);
  const env = options.env ?? process.env;
  const platform = options.platform ?? process.platform;
  const home = path.resolve(env.HOME || env.USERPROFILE || homedir());
  const baseDir = options.baseDir ? path.resolve(options.baseDir) : "";
  const authorSegment = sanitizePathSegment(options.appAuthor || appName);
  const appSegment = sanitizePathSegment(appName);
  const defaults = defaultDirs(platform, env, home, authorSegment, appSegment, baseDir);
  return {
    home,
    data: path.resolve(options.dirs?.data ?? defaults.data),
    config: path.resolve(options.dirs?.config ?? defaults.config),
    cache: path.resolve(options.dirs?.cache ?? defaults.cache),
    logs: path.resolve(options.dirs?.logs ?? defaults.logs),
    temp: path.resolve(options.dirs?.temp ?? defaults.temp),
  };
}

export class LocalFileStore {
  readonly root: string;
  readonly label: string;

  constructor(root: string, options: LocalFileStoreOptions = {}) {
    this.root = path.resolve(root);
    this.label = options.label ?? "local";
  }

  child(relativePath: string, options: LocalFileStoreOptions = {}): LocalFileStore {
    return new LocalFileStore(this.resolvePath(relativePath), { label: options.label ?? this.label });
  }

  async ensure(): Promise<void> {
    await mkdir(this.root, { recursive: true });
  }

  resolvePath(relativePath = "."): string {
    const clean = normalizeRelativePath(relativePath);
    const fullPath = path.resolve(this.root, clean);
    assertInsideRoot(this.root, fullPath);
    return fullPath;
  }

  relativePath(fullPath: string): string {
    const absolute = path.resolve(fullPath);
    assertInsideRoot(this.root, absolute);
    return toPortablePath(path.relative(this.root, absolute));
  }

  async exists(relativePath = "."): Promise<boolean> {
    try {
      await access(this.resolvePath(relativePath), fsConstants.F_OK);
      return true;
    } catch {
      return false;
    }
  }

  async stat(relativePath = "."): Promise<LocalFileStat> {
    const fullPath = this.resolvePath(relativePath);
    const info = await stat(fullPath);
    return {
      path: toPortablePath(path.relative(this.root, fullPath)) || ".",
      fullPath,
      type: fileType(info),
      size: info.size,
      modifiedAt: info.mtime,
    };
  }

  async mkdir(relativePath = "."): Promise<string> {
    const fullPath = this.resolvePath(relativePath);
    await mkdir(fullPath, { recursive: true });
    return fullPath;
  }

  async list(relativePath = ".", options: LocalListOptions = {}): Promise<LocalFileStat[]> {
    const base = this.resolvePath(relativePath);
    const maxDepth = options.recursive ? options.maxDepth ?? Number.POSITIVE_INFINITY : 1;
    const out: LocalFileStat[] = [];
    await this.walk(this.root, base, base, out, maxDepth, options);
    return out.sort((a, b) => a.path.localeCompare(b.path));
  }

  async listEntries(relativePath = ".", options: LocalListOptions = {}): Promise<LocalEntryList> {
    const stats = await this.list(relativePath, { ...options, includeDirectories: options.includeDirectories ?? true });
    return { object: "list", entries: stats.map(localEntryFromStat) };
  }

  async searchEntries(params: LocalSearchEntriesParams): Promise<LocalEntryList> {
    const query = params.query.trim().toLowerCase();
    if (!query) {
      throw new Error("query is required");
    }
    const limit = positiveInt(params.limit, 100);
    const stats = await this.list(params.path ?? ".", { recursive: true, includeDirectories: true });
    const entries = stats
      .filter((item) => item.path.toLowerCase().includes(query))
      .slice(0, limit)
      .map(localEntryFromStat);
    return { object: "list", entries };
  }

  async readText(relativePath: string): Promise<string> {
    return await readFile(this.resolvePath(relativePath), "utf8");
  }

  async readJSON<T = unknown>(relativePath: string): Promise<T> {
    return JSON.parse(await this.readText(relativePath)) as T;
  }

  async readBytes(relativePath: string): Promise<Uint8Array> {
    return await readFile(this.resolvePath(relativePath));
  }

  async readFile(relativePath: string, params: LocalReadFileRawParams): Promise<LocalFileRaw>;
  async readFile(relativePath: string, params?: LocalReadFileParams): Promise<LocalFileDeliver>;
  async readFile(
    relativePath: string,
    params: LocalReadFileParams | LocalReadFileRawParams = {},
  ): Promise<LocalFileDeliver | LocalFileRaw> {
    const fullPath = this.resolvePath(relativePath);
    const info = await stat(fullPath);
    if (!info.isFile()) {
      throw new LocalPathError("local path is not a file", relativePath);
    }
    const maxBytes = params.maxBytes ?? Number.POSITIVE_INFINITY;
    const raw = await readFile(fullPath);
    const truncated = Number.isFinite(maxBytes) && raw.byteLength > maxBytes;
    const content = truncated ? raw.subarray(0, maxBytes) : raw;
    const portablePath = toPortablePath(path.relative(this.root, fullPath));
    if ("format" in params && params.format === "raw") {
      return {
        path: portablePath,
        size: info.size,
        truncated,
        content,
        content_type: mimeTypeForPath(portablePath),
      };
    }
    if (looksBinary(content)) {
      return {
        path: portablePath,
        encoding: "base64",
        mime_type: mimeTypeForPath(portablePath),
        size: info.size,
        truncated,
        content_base64: Buffer.from(content).toString("base64"),
      };
    }
    return {
      path: portablePath,
      encoding: "text",
      mime_type: mimeTypeForPath(portablePath),
      size: info.size,
      truncated,
      content: content.toString("utf8"),
    };
  }

  async writeText(relativePath: string, content: string, options: { atomic?: boolean } = {}): Promise<string> {
    const fullPath = this.resolvePath(relativePath);
    await mkdir(path.dirname(fullPath), { recursive: true });
    if (options.atomic === false) {
      await writeFile(fullPath, content, "utf8");
      return fullPath;
    }
    await atomicWrite(fullPath, content);
    return fullPath;
  }

  async writeJSON(relativePath: string, value: unknown, options: { pretty?: boolean; atomic?: boolean } = {}): Promise<string> {
    const spaces = options.pretty === false ? 0 : 2;
    const text = JSON.stringify(value, null, spaces) + "\n";
    return await this.writeText(relativePath, text, { atomic: options.atomic });
  }

  async writeBytes(relativePath: string, content: Uint8Array, options: { atomic?: boolean } = {}): Promise<string> {
    const fullPath = this.resolvePath(relativePath);
    await mkdir(path.dirname(fullPath), { recursive: true });
    if (options.atomic === false) {
      await writeFile(fullPath, content);
      return fullPath;
    }
    await atomicWrite(fullPath, content);
    return fullPath;
  }

  async writeFile(relativePath: string, content: string | Uint8Array, options: { atomic?: boolean } = {}): Promise<LocalFileWrite> {
    const fullPath = typeof content === "string"
      ? await this.writeText(relativePath, content, options)
      : await this.writeBytes(relativePath, content, options);
    const info = await stat(fullPath);
    return { path: toPortablePath(path.relative(this.root, fullPath)), size: info.size };
  }

  async remove(relativePath: string): Promise<void> {
    await rm(this.resolvePath(relativePath), { recursive: true, force: true });
  }

  async deletePath(relativePath: string): Promise<LocalPathDelete> {
    await this.remove(relativePath);
    return { path: normalizeRelativePath(relativePath), recursive: true };
  }

  async createDirectory(relativePath = "."): Promise<{ path: string }> {
    await this.mkdir(relativePath);
    return { path: normalizeRelativePath(relativePath) };
  }

  async copy(fromRelativePath: string, toRelativePath: string): Promise<string> {
    const from = this.resolvePath(fromRelativePath);
    const to = this.resolvePath(toRelativePath);
    await mkdir(path.dirname(to), { recursive: true });
    await copyFile(from, to);
    return to;
  }

  async readLines(relativePath: string, params: LocalReadLinesParams): Promise<LocalFileLines> {
    const startLine = Math.trunc(params.startLine);
    const endLine = params.endLine == null ? undefined : Math.trunc(params.endLine);
    const file = await this.readFile(relativePath, { maxBytes: params.maxBytes, format: "raw" });
    if (looksBinary(file.content)) {
      throw new LocalNotTextFileError(relativePath);
    }
    const text = Buffer.from(file.content).toString("utf8");
    const all = splitLines(text);
    const selected = selectLineRange(all, startLine, endLine);
    return {
      path: file.path,
      start_line: startLine,
      end_line: selected.endLine,
      total_lines: all.length,
      lines: selected.lines,
      file_truncated: file.truncated,
      size: file.size,
    };
  }

  async patchLines(relativePath: string, params: LocalPatchLinesParams): Promise<LocalFileLinesPatch> {
    const startLine = Math.trunc(params.startLine);
    const endLine = params.endLine == null ? undefined : Math.trunc(params.endLine);
    const file = await this.readFile(relativePath, { maxBytes: params.maxBytes, format: "raw" });
    if (file.truncated) {
      throw new LocalFileTooLargeError(relativePath);
    }
    if (looksBinary(file.content)) {
      throw new LocalNotTextFileError(relativePath);
    }
    const original = Buffer.from(file.content).toString("utf8");
    const patched = patchLineRange(original, startLine, endLine, params.replacement ?? "");
    const written = await this.writeFile(relativePath, patched.content);
    return {
      path: file.path,
      start_line: startLine,
      end_line: patched.endLine,
      total_lines: patched.totalLines,
      size: written.size,
    };
  }

  async grep(params: LocalGrepParams): Promise<LocalGrepResponse> {
    const pattern = params.pattern.trim();
    if (!pattern) {
      throw new Error("pattern is required");
    }
    const limit = positiveInt(params.limit, 200);
    const maxFiles = positiveInt(params.maxFiles, 500);
    const maxBytesPerFile = positiveInt(params.maxBytesPerFile, 512 * 1024);
    const maxLineLength = positiveInt(params.maxLineLength, 500);
    const stats = await this.grepCandidates(params.path ?? ".", params.ignore);
    const matches: LocalGrepMatch[] = [];
    let filesScanned = 0;
    let scanTruncated = false;
    for (const item of stats) {
      if (matches.length >= limit || filesScanned >= maxFiles) {
        scanTruncated = true;
        break;
      }
      if (item.type !== "file" || item.size > maxBytesPerFile || !isLikelyTextFile(item.path)) {
        continue;
      }
      const raw = await readOptionalFile(item.fullPath);
      if (!raw) {
        continue;
      }
      if (looksBinary(raw)) {
        continue;
      }
      filesScanned++;
      const lines = splitLines(Buffer.from(raw).toString("utf8"));
      for (let i = 0; i < lines.length; i++) {
        if (!lines[i].includes(pattern)) {
          continue;
        }
        matches.push({
          path: item.path,
          line_number: i + 1,
          line: lines[i].length > maxLineLength ? lines[i].slice(0, maxLineLength) : lines[i],
        });
        if (matches.length >= limit) {
          scanTruncated = true;
          break;
        }
      }
    }
    return { object: "list", matches, files_scanned: filesScanned, scan_truncated: scanTruncated };
  }

  private async grepCandidates(relativePath: string, ignore?: LocalListOptions["ignore"]): Promise<LocalFileStat[]> {
    const fullPath = this.resolvePath(relativePath);
    const info = await stat(fullPath);
    const portablePath = toPortablePath(path.relative(this.root, fullPath)) || ".";
    if (ignored(portablePath, ignore)) {
      return [];
    }
    if (info.isFile()) {
      return [{
        path: portablePath,
        fullPath,
        type: "file",
        size: info.size,
        modifiedAt: info.mtime,
      }];
    }
    if (info.isDirectory()) {
      return await this.list(relativePath, { recursive: true, ignore });
    }
    return [];
  }

  async summarize(params: LocalSummarizeParams = {}): Promise<LocalSummary> {
    const maxFiles = positiveInt(params.maxFiles, 2000);
    const maxPreviews = positiveInt(params.maxPreviews, 20);
    const previewBytes = positiveInt(params.previewBytes, 4096);
    const topPaths = positiveInt(params.topPaths, 20);
    const stats = await this.list(params.path ?? ".", { recursive: true, ignore: params.ignore });
    const files = stats.filter((item) => item.type === "file").slice(0, maxFiles);
    const scanTruncated = stats.filter((item) => item.type === "file").length > files.length;
    const totalBytes = files.reduce((sum, item) => sum + item.size, 0);
    const bySize = [...files].sort((a, b) => (b.size !== a.size ? b.size - a.size : a.path.localeCompare(b.path)));
    const previews: LocalSummaryPreview[] = [];
    for (const item of bySize) {
      if (previews.length >= maxPreviews) {
        break;
      }
      if (!isLikelyTextFile(item.path) || item.size > previewBytes * 4) {
        continue;
      }
      const raw = await readOptionalFile(item.fullPath);
      if (!raw) {
        continue;
      }
      if (looksBinary(raw)) {
        continue;
      }
      const truncated = raw.byteLength > previewBytes;
      previews.push({
        path: item.path,
        size: item.size,
        preview: Buffer.from(raw.subarray(0, previewBytes)).toString("utf8"),
        preview_truncated: truncated || undefined,
      });
    }
    return {
      summary_path: "",
      file_count: files.length,
      total_bytes: totalBytes,
      top_paths_by_size: bySize.slice(0, topPaths).map((item) => `${item.path} (${item.size} bytes)`),
      text_previews: previews,
      generated_at_unix: Math.floor(Date.now() / 1000),
      scan_truncated: scanTruncated,
    };
  }

  private async walk(
    storeRoot: string,
    scanRoot: string,
    dir: string,
    out: LocalFileStat[],
    maxDepth: number,
    options: LocalListOptions,
  ): Promise<void> {
    const entries = await readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      const relativePath = toPortablePath(path.relative(storeRoot, fullPath));
      if (ignored(relativePath, options.ignore)) {
        continue;
      }
      const info = await lstatOptional(fullPath);
      if (!info) {
        continue;
      }
      const item: LocalFileStat = {
        path: relativePath,
        fullPath,
        type: fileType(info),
        size: info.size,
        modifiedAt: info.mtime,
      };
      if (info.isDirectory()) {
        if (options.includeDirectories) {
          out.push(item);
        }
        const depth = toPortablePath(path.relative(scanRoot, fullPath)).split("/").filter(Boolean).length;
        if (depth < maxDepth) {
          await this.walk(storeRoot, scanRoot, fullPath, out, maxDepth, options);
        }
        continue;
      }
      out.push(item);
    }
  }
}

export class LocalConfigStore {
  constructor(readonly files: LocalFileStore) {}

  async read<T = Record<string, unknown>>(name = "settings.json", fallback?: T): Promise<T> {
    try {
      return await this.files.readJSON<T>(name);
    } catch (error: any) {
      if (error?.code === "ENOENT" && arguments.length >= 2) {
        return fallback as T;
      }
      throw error;
    }
  }

  async write(name: string, value: unknown): Promise<string> {
    return await this.files.writeJSON(name, value);
  }

  async get<T = unknown>(name: string, key: string, fallback?: T): Promise<T> {
    const config = await this.readRecord(name);
    return (key in config ? config[key] : fallback) as T;
  }

  async set(name: string, key: string, value: unknown): Promise<string> {
    const config = await this.readRecord(name);
    config[key] = value;
    return await this.write(name, config);
  }

  async delete(name: string, key: string): Promise<string> {
    const config = await this.readRecord(name);
    delete config[key];
    return await this.write(name, config);
  }

  private async readRecord(name: string): Promise<Record<string, unknown>> {
    const value = await this.read<unknown>(name, {});
    if (!value || typeof value !== "object" || Array.isArray(value)) {
      throw new LocalConfigError(`local config ${name} must contain a JSON object`, name);
    }
    return value as Record<string, unknown>;
  }
}

export class LocalWorkdirManager {
  open(root: string, options: LocalWorkdirOptions = {}): LocalWorkdir {
    return new LocalWorkdir(root, options);
  }
}

export class LocalWorkdir {
  readonly root: string;
  readonly name: string;
  readonly metadata: Record<string, unknown>;
  readonly trusted: boolean;
  readonly files: LocalFileStore;
  readonly ignore: LocalIgnoreRule[];
  readonly gitignore: boolean;
  readonly maxFileBytes: number;

  constructor(root: string, options: LocalWorkdirOptions = {}) {
    this.root = path.resolve(root);
    this.name = options.name?.trim() || path.basename(this.root) || "workdir";
    this.metadata = { ...(options.metadata ?? {}) };
    this.trusted = options.trusted ?? false;
    this.files = new LocalFileStore(this.root, { label: "workdir" });
    this.ignore = [...defaultWorkdirIgnoreRules(), ...(options.ignore ?? [])];
    this.gitignore = options.gitignore ?? true;
    this.maxFileBytes = positiveInt(options.maxFileBytes, 10 * 1024 * 1024);
  }

  async ensure(): Promise<void> {
    await this.files.ensure();
    if (this.gitignore) {
      await this.loadIgnoreFiles();
    }
  }

  async loadIgnoreFiles(files = [".gitignore"]): Promise<LocalIgnoreRule[]> {
    const loaded: LocalIgnoreRule[] = [];
    for (const file of files) {
      try {
        const text = await this.files.readText(file);
        loaded.push(...parseIgnoreFile(text));
      } catch (error: any) {
        if (error?.code !== "ENOENT") {
          throw error;
        }
      }
    }
    this.ignore.push(...loaded);
    return loaded;
  }

  child(relativePath: string, options: LocalWorkdirOptions = {}): LocalWorkdir {
    return new LocalWorkdir(this.files.resolvePath(relativePath), {
      name: options.name,
      metadata: options.metadata ?? this.metadata,
      trusted: options.trusted ?? this.trusted,
      ignore: options.ignore ?? this.ignore,
      gitignore: options.gitignore ?? this.gitignore,
      maxFileBytes: options.maxFileBytes ?? this.maxFileBytes,
    });
  }

  resolvePath(relativePath = "."): string {
    this.assertAllowed(relativePath);
    return this.files.resolvePath(relativePath);
  }

  async list(relativePath = ".", options: LocalListOptions = {}): Promise<LocalFileStat[]> {
    return await this.files.list(relativePath, this.scopedListOptions(options));
  }

  async listEntries(relativePath = ".", options: LocalListOptions = {}): Promise<LocalEntryList> {
    return await this.files.listEntries(relativePath, this.scopedListOptions(options));
  }

  async searchEntries(params: LocalSearchEntriesParams): Promise<LocalEntryList> {
    const query = params.query.trim().toLowerCase();
    if (!query) {
      throw new Error("query is required");
    }
    const limit = positiveInt(params.limit, 100);
    const stats = await this.list(params.path ?? ".", { recursive: true, includeDirectories: true });
    return {
      object: "list",
      entries: stats
        .filter((item) => item.path.toLowerCase().includes(query))
        .slice(0, limit)
        .map(localEntryFromStat),
    };
  }

  async readFile(relativePath: string, params: LocalReadFileRawParams): Promise<LocalFileRaw>;
  async readFile(relativePath: string, params?: LocalReadFileParams): Promise<LocalFileDeliver>;
  async readFile(relativePath: string, params: LocalReadFileParams | LocalReadFileRawParams = {}): Promise<LocalFileDeliver | LocalFileRaw> {
    this.assertAllowed(relativePath);
    return await this.files.readFile(relativePath, { maxBytes: params.maxBytes ?? this.maxFileBytes, ...params } as any);
  }

  async writeFile(relativePath: string, content: string | Uint8Array, options: { atomic?: boolean } = {}): Promise<LocalFileWrite> {
    this.assertAllowed(relativePath);
    return await this.files.writeFile(relativePath, content, options);
  }

  async readText(relativePath: string): Promise<string> {
    this.assertAllowed(relativePath);
    return await this.files.readText(relativePath);
  }

  async writeText(relativePath: string, content: string, options: { atomic?: boolean } = {}): Promise<string> {
    this.assertAllowed(relativePath);
    return await this.files.writeText(relativePath, content, options);
  }

  async deletePath(relativePath: string): Promise<LocalPathDelete> {
    this.assertAllowed(relativePath);
    return await this.files.deletePath(relativePath);
  }

  async createDirectory(relativePath = "."): Promise<{ path: string }> {
    this.assertAllowed(relativePath);
    return await this.files.createDirectory(relativePath);
  }

  async readLines(relativePath: string, params: LocalReadLinesParams): Promise<LocalFileLines> {
    this.assertAllowed(relativePath);
    return await this.files.readLines(relativePath, { maxBytes: params.maxBytes ?? this.maxFileBytes, ...params });
  }

  async previewPatchLines(relativePath: string, params: LocalPatchLinesParams): Promise<LocalLinePatchPreview> {
    this.assertAllowed(relativePath);
    const lines = await this.readLines(relativePath, {
      startLine: params.startLine,
      endLine: params.endLine,
      maxBytes: params.maxBytes ?? this.maxFileBytes,
    });
    const file = await this.files.readFile(relativePath, { maxBytes: params.maxBytes ?? this.maxFileBytes, format: "raw" });
    if (file.truncated) {
      throw new Error("local file is too large to patch");
    }
    if (looksBinary(file.content)) {
      throw new Error("local file must be text");
    }
    const patched = patchLineRange(Buffer.from(file.content).toString("utf8"), params.startLine, params.endLine, params.replacement ?? "");
    return {
      path: lines.path,
      start_line: lines.start_line,
      end_line: lines.end_line,
      total_lines: patched.totalLines,
      before: lines.lines,
      after: params.replacement === "" ? [] : splitLines(params.replacement ?? ""),
    };
  }

  async patchLines(relativePath: string, params: LocalPatchLinesParams): Promise<LocalFileLinesPatch> {
    this.assertAllowed(relativePath);
    return await this.files.patchLines(relativePath, { maxBytes: params.maxBytes ?? this.maxFileBytes, ...params });
  }

  async previewEdits(edits: LocalWorkdirLineEdit[]): Promise<LocalWorkdirEditPlan> {
    const previews: LocalLinePatchPreview[] = [];
    for (const edit of edits) {
      await this.assertExpectedHash(edit.path, edit.expectedSha256);
      previews.push(await this.previewPatchLines(edit.path, edit));
    }
    return { edits: edits.map((edit) => ({ ...edit })), previews };
  }

  async applyEdits(edits: LocalWorkdirLineEdit[]): Promise<LocalWorkdirEditResult> {
    const backups: Array<{ path: string; content: string }> = [];
    const applied: LocalFileLinesPatch[] = [];
    try {
      for (const edit of edits) {
        await this.assertExpectedHash(edit.path, edit.expectedSha256);
        backups.push({ path: edit.path, content: await this.readText(edit.path) });
        applied.push(await this.patchLines(edit.path, edit));
      }
    } catch (error) {
      for (const backup of backups.reverse()) {
        await this.writeText(backup.path, backup.content);
      }
      throw error;
    }
    return { applied, backups };
  }

  classifyPath(relativePath: string): LocalPathSensitivityInfo {
    return classifyLocalPathSensitivity(relativePath);
  }

  async grep(params: LocalGrepParams): Promise<LocalGrepResponse> {
    return await this.files.grep({ ...params, ignore: this.mergeIgnore(params.ignore) });
  }

  async summarize(params: LocalSummarizeParams = {}): Promise<LocalSummary> {
    return await this.files.summarize({ ...params, ignore: this.mergeIgnore(params.ignore) });
  }

  async snapshot(params: LocalWorkdirSnapshotParams = {}): Promise<LocalWorkdirSnapshot> {
    const maxBytes = positiveInt(params.maxBytesPerFile, this.maxFileBytes);
    const shouldHash = params.hash ?? true;
    const stats = await this.list(params.path ?? ".", { recursive: true });
    const files: LocalWorkdirSnapshotFile[] = [];
    for (const item of stats) {
      if (item.type !== "file") {
        continue;
      }
      const snap: LocalWorkdirSnapshotFile = {
        path: item.path,
        size: item.size,
        modified_at_unix: Math.floor(item.modifiedAt.getTime() / 1000),
      };
      if (shouldHash && item.size <= maxBytes) {
        snap.sha256 = createHash("sha256").update(await readFile(item.fullPath)).digest("hex");
      }
      files.push(snap);
    }
    return {
      root: this.root,
      name: this.name,
      generated_at_unix: Math.floor(Date.now() / 1000),
      files,
    };
  }

  diff(before: LocalWorkdirSnapshot, after: LocalWorkdirSnapshot): LocalWorkdirDiff {
    const beforeByPath = new Map(before.files.map((file) => [file.path, file]));
    const afterByPath = new Map(after.files.map((file) => [file.path, file]));
    const added: LocalWorkdirSnapshotFile[] = [];
    const modified: Array<{ before: LocalWorkdirSnapshotFile; after: LocalWorkdirSnapshotFile }> = [];
    const deleted: LocalWorkdirSnapshotFile[] = [];
    const unchanged: LocalWorkdirSnapshotFile[] = [];
    for (const afterFile of after.files) {
      const beforeFile = beforeByPath.get(afterFile.path);
      if (!beforeFile) {
        added.push(afterFile);
      } else if (snapshotFileChanged(beforeFile, afterFile)) {
        modified.push({ before: beforeFile, after: afterFile });
      } else {
        unchanged.push(afterFile);
      }
    }
    for (const beforeFile of before.files) {
      if (!afterByPath.has(beforeFile.path)) {
        deleted.push(beforeFile);
      }
    }
    return { added, modified, deleted, unchanged };
  }

  watch(
    onEvent: (event: LocalWorkdirWatchEvent) => void,
    options: { recursive?: boolean } = {},
  ): LocalWorkdirWatcher {
    const watcher = watchFS(this.root, { recursive: options.recursive ?? process.platform !== "linux" }, (eventType, filename) => {
      const rel = filename ? toPortablePath(String(filename)) : ".";
      if (ignored(rel, this.ignore)) {
        return;
      }
      onEvent({ type: eventType === "rename" ? "rename" : "change", path: rel });
    });
    return { close: () => watcher.close() };
  }

  private scopedListOptions(options: LocalListOptions): LocalListOptions {
    return { ...options, ignore: this.mergeIgnore(options.ignore) };
  }

  private mergeIgnore(ignore: LocalListOptions["ignore"]): LocalIgnoreRule[] {
    return [...this.ignore, ...(ignore ?? [])];
  }

  private assertAllowed(relativePath: string): void {
    const rel = normalizeRelativePath(relativePath);
    if (rel !== "." && ignored(rel, this.ignore)) {
      throw new LocalIgnoredPathError(rel);
    }
  }

  private async assertExpectedHash(relativePath: string, expectedSha256?: string): Promise<void> {
    if (!expectedSha256) {
      return;
    }
    const raw = await readFile(this.files.resolvePath(relativePath));
    const actual = createHash("sha256").update(raw).digest("hex");
    if (actual !== expectedSha256) {
      throw new LocalError("local_edit_conflict", `local file changed before edit: ${relativePath}`, { path: relativePath });
    }
  }
}

export class LocalSkillStore {
  constructor(readonly files: LocalFileStore) {}

  async fromDirectory(rootDir: string, options: LocalSkillDirectoryOptions = {}): Promise<LocalSkillDescriptor> {
    return await localSkillFromDirectory(rootDir, options);
  }

  async discover(options: LocalSkillDiscoveryOptions = {}): Promise<LocalSkillDescriptor[]> {
    const roots = options.roots?.length ? options.roots : [this.files.root];
    const skillDirs: string[] = [];
    for (const root of roots) {
      skillDirs.push(...await discoverSkillDirectories(path.resolve(root), options));
    }
    const unique = [...new Set(skillDirs)].sort((a, b) => a.localeCompare(b));
    return await Promise.all(unique.map((dir) => localSkillFromDirectory(dir, options)));
  }
}

function defaultDirs(
  platform: NodeJS.Platform,
  env: Record<string, string | undefined>,
  home: string,
  authorSegment: string,
  appSegment: string,
  baseDir: string,
): Omit<LocalAppDirs, "home"> {
  if (baseDir) {
    return {
      data: path.join(baseDir, "data"),
      config: path.join(baseDir, "config"),
      cache: path.join(baseDir, "cache"),
      logs: path.join(baseDir, "logs"),
      temp: path.join(baseDir, "tmp"),
    };
  }
  if (platform === "darwin") {
    return {
      data: path.join(home, "Library", "Application Support", appSegment),
      config: path.join(home, "Library", "Application Support", appSegment),
      cache: path.join(home, "Library", "Caches", appSegment),
      logs: path.join(home, "Library", "Logs", appSegment),
      temp: path.join(tmpdir(), appSegment),
    };
  }
  if (platform === "win32") {
    const roaming = env.APPDATA || path.join(home, "AppData", "Roaming");
    const local = env.LOCALAPPDATA || path.join(home, "AppData", "Local");
    return {
      data: path.join(roaming, authorSegment, appSegment),
      config: path.join(roaming, authorSegment, appSegment),
      cache: path.join(local, authorSegment, appSegment, "Cache"),
      logs: path.join(local, authorSegment, appSegment, "Logs"),
      temp: path.join(tmpdir(), appSegment),
    };
  }
  return {
    data: path.join(env.XDG_DATA_HOME || path.join(home, ".local", "share"), appSegment),
    config: path.join(env.XDG_CONFIG_HOME || path.join(home, ".config"), appSegment),
    cache: path.join(env.XDG_CACHE_HOME || path.join(home, ".cache"), appSegment),
    logs: path.join(env.XDG_STATE_HOME || path.join(home, ".local", "state"), appSegment, "logs"),
    temp: path.join(tmpdir(), appSegment),
  };
}

async function discoverSkillDirectories(root: string, options: LocalSkillDiscoveryOptions): Promise<string[]> {
  try {
    const info = await stat(root);
    if (!info.isDirectory()) {
      return [];
    }
  } catch (error: any) {
    if (error?.code === "ENOENT") {
      return [];
    }
    throw error;
  }
  const out: string[] = [];
  const maxDepth = options.recursive ? options.maxDepth ?? Number.POSITIVE_INFINITY : 1;
  await walkSkillDirectories(root, root, out, maxDepth);
  return out;
}

async function walkSkillDirectories(root: string, dir: string, out: string[], maxDepth: number): Promise<void> {
  if (await fileExists(path.join(dir, "SKILL.md"))) {
    out.push(dir);
  }
  const relative = path.relative(root, dir);
  const depth = relative ? relative.split(path.sep).length : 0;
  if (depth >= maxDepth) {
    return;
  }
  const entries = await readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (!entry.isDirectory() || ignoredDirectoryName(entry.name)) {
      continue;
    }
    await walkSkillDirectories(root, path.join(dir, entry.name), out, maxDepth);
  }
}

async function fileExists(fullPath: string): Promise<boolean> {
  try {
    const info = await stat(fullPath);
    return info.isFile();
  } catch {
    return false;
  }
}

async function lstatOptional(fullPath: string) {
  try {
    return await lstat(fullPath);
  } catch (error: any) {
    if (error?.code === "ENOENT" || error?.code === "ENOTDIR") {
      return null;
    }
    throw error;
  }
}

async function readOptionalFile(fullPath: string): Promise<Uint8Array | null> {
  try {
    return await readFile(fullPath);
  } catch (error: any) {
    if (error?.code === "ENOENT" || error?.code === "ENOTDIR" || error?.code === "EISDIR") {
      return null;
    }
    throw error;
  }
}

function normalizeAppName(appName: string): string {
  const trimmed = appName.trim();
  if (!trimmed) {
    throw new LocalConfigError("appName is required");
  }
  return trimmed;
}

function sanitizePathSegment(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "") || "agent-api";
}

function normalizeRelativePath(value: string): string {
  const trimmed = value.trim();
  if (!trimmed || trimmed === ".") {
    return ".";
  }
  if (path.isAbsolute(trimmed)) {
    throw new LocalPathError("local path must be relative", value);
  }
  return trimmed;
}

function assertInsideRoot(root: string, fullPath: string): void {
  const relative = path.relative(root, fullPath);
  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new LocalPathError("local path must stay inside the store root", fullPath);
  }
}

function toPortablePath(value: string): string {
  return value.split(path.sep).join("/");
}

function fileType(info: { isFile(): boolean; isDirectory(): boolean; isSymbolicLink(): boolean }): LocalFileType {
  if (info.isFile()) return "file";
  if (info.isDirectory()) return "directory";
  if (info.isSymbolicLink()) return "symlink";
  return "other";
}

function localEntryFromStat(item: LocalFileStat): LocalEntry {
  return {
    path: item.path,
    is_dir: item.type === "directory",
    size: item.type === "directory" ? 0 : item.size,
    modified_at_unix: Math.floor(item.modifiedAt.getTime() / 1000),
  };
}

async function atomicWrite(fullPath: string, content: string | Uint8Array): Promise<void> {
  const tmpPath = path.join(path.dirname(fullPath), `.${path.basename(fullPath)}.${process.pid}.${randomUUID()}.tmp`);
  await writeFile(tmpPath, content);
  await rename(tmpPath, fullPath);
}

function ignored(relativePath: string, ignore: LocalListOptions["ignore"]): boolean {
  if (!ignore?.length) {
    return false;
  }
  return ignore.some((rule) => {
    if (typeof rule === "string") {
      return relativePath === rule || relativePath.startsWith(`${rule}/`) || relativePath.endsWith(`/${rule}`) || relativePath.includes(`/${rule}/`);
    }
    if (rule instanceof RegExp) return rule.test(relativePath);
    return rule(relativePath);
  });
}

function ignoredDirectoryName(name: string): boolean {
  return name === ".git" || name === "node_modules" || name === "__pycache__";
}

function defaultWorkdirIgnoreRules(): LocalIgnoreRule[] {
  return [
    ".git",
    "node_modules",
    "__pycache__",
    ".DS_Store",
    "dist",
    "build",
    "coverage",
    ".next",
    ".turbo",
    ".cache",
    /\.pyc$/,
    /\.pyo$/,
    /\.class$/,
    /\.log$/,
  ];
}

function parseIgnoreFile(text: string): LocalIgnoreRule[] {
  const rules: LocalIgnoreRule[] = [];
  for (const rawLine of text.split(/\r?\n/)) {
    let line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }
    if (line.startsWith("!")) {
      continue;
    }
    line = line.replace(/\\/g, "/");
    if (line.startsWith("/")) {
      line = line.slice(1);
    }
    if (line.endsWith("/")) {
      line = line.slice(0, -1);
    }
    if (!line || line.includes("*")) {
      rules.push(globIgnoreRule(line));
      continue;
    }
    rules.push(line);
  }
  return rules;
}

function globIgnoreRule(pattern: string): (relativePath: string) => boolean {
  const escaped = pattern
    .split("*")
    .map((part) => part.replace(/[.+?^${}()|[\]\\]/g, "\\$&"))
    .join(".*");
  const regex = new RegExp(`(^|/)${escaped}($|/)`);
  return (relativePath) => regex.test(relativePath);
}

function snapshotFileChanged(before: LocalWorkdirSnapshotFile, after: LocalWorkdirSnapshotFile): boolean {
  if (before.sha256 || after.sha256) {
    return before.sha256 !== after.sha256;
  }
  return before.size !== after.size || before.modified_at_unix !== after.modified_at_unix;
}

export function classifyLocalPathSensitivity(relativePath: string): LocalPathSensitivityInfo {
  const rel = normalizeRelativePath(relativePath);
  const lower = rel.toLowerCase();
  const base = path.basename(lower);
  if (
    base === ".env" ||
    base.startsWith(".env.") ||
    lower.includes("id_rsa") ||
    lower.includes("id_ed25519") ||
    lower.endsWith(".pem") ||
    lower.endsWith(".key") ||
    lower.endsWith(".p12") ||
    lower.endsWith(".pfx")
  ) {
    return { path: rel, sensitivity: "secret", reason: "path commonly contains credentials or private keys" };
  }
  if (
    lower.includes("secret") ||
    lower.includes("token") ||
    lower.includes("credential") ||
    lower.includes("password") ||
    lower.endsWith(".crt") ||
    lower.endsWith(".cert")
  ) {
    return { path: rel, sensitivity: "sensitive", reason: "path name suggests sensitive material" };
  }
  return { path: rel, sensitivity: "normal" };
}

function positiveInt(value: number | undefined, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

function looksBinary(content: Uint8Array): boolean {
  return content.includes(0);
}

const textExtensions = new Set([
  ".txt", ".md", ".markdown", ".json", ".yaml", ".yml", ".toml", ".xml", ".csv",
  ".py", ".go", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".html", ".htm",
  ".css", ".scss", ".sh", ".bash", ".zsh", ".sql", ".env", ".ini", ".cfg",
  ".conf", ".log", ".rst", ".adoc", ".gradle", ".properties", ".mod", ".sum",
  ".dockerfile",
]);

function isLikelyTextFile(relativePath: string): boolean {
  const lower = relativePath.toLowerCase();
  const ext = path.extname(lower);
  return !ext || textExtensions.has(ext) || lower.endsWith("dockerfile");
}

function mimeTypeForPath(relativePath: string): string {
  const ext = path.extname(relativePath).toLowerCase();
  switch (ext) {
    case ".json":
      return "application/json";
    case ".md":
    case ".markdown":
      return "text/markdown";
    case ".html":
    case ".htm":
      return "text/html";
    case ".css":
      return "text/css";
    case ".csv":
      return "text/csv";
    case ".xml":
      return "application/xml";
    case ".png":
      return "image/png";
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".gif":
      return "image/gif";
    case ".svg":
      return "image/svg+xml";
    case ".pdf":
      return "application/pdf";
    default:
      return isLikelyTextFile(relativePath) ? "text/plain" : "application/octet-stream";
  }
}

function splitLines(content: string): string[] {
  if (content.length === 0) {
    return [""];
  }
  let text = content;
  if (text.endsWith("\n")) {
    text = text.slice(0, -1);
    if (text === "") {
      return [""];
    }
  }
  return text.split("\n");
}

function selectLineRange(lines: string[], startLine: number, endLine?: number): { lines: string[]; endLine: number } {
  if (startLine < 1) {
    throw new Error("startLine must be >= 1");
  }
  const total = lines.length || 1;
  const end = endLine == null || endLine <= 0 ? total : Math.min(endLine, total);
  if (startLine > total || end < startLine) {
    throw new Error("invalid line range");
  }
  return { lines: lines.slice(startLine - 1, end), endLine: end };
}

function patchLineRange(
  content: string,
  startLine: number,
  endLine: number | undefined,
  replacement: string,
): { content: string; endLine: number; totalLines: number } {
  const lines = splitLines(content);
  const selected = selectLineRange(lines, startLine, endLine);
  const replacementLines = replacement === "" ? [] : splitLines(replacement);
  const patched = [
    ...lines.slice(0, startLine - 1),
    ...replacementLines,
    ...lines.slice(selected.endLine),
  ];
  let out = patched.join("\n");
  if (content.endsWith("\n")) {
    out += "\n";
  }
  return { content: out, endLine: selected.endLine, totalLines: patched.length };
}
