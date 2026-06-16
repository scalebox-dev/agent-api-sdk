import { buildQuery } from "../internal/query.js";
import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type {
  CreateSkillParams,
  CreateSkillDevParams,
  CreateSkillDevResponse,
  DiscoverSkillsParams,
  FocusSkillParams,
  ImportSkillArchiveParams,
  ListSkillFilesParams,
  ListSkillFilesResponse,
  ListSkillsParams,
  ListSkillsResponse,
  ListSkillSummariesResponse,
  PullSkillDirectoryParams,
  PushSkillDirectoryParams,
  Skill,
  SkillAcceptStrategy,
  SkillArchive,
  SkillArchiveParams,
  SkillBranchDiff,
  SkillBranchDiffParams,
  SkillDirectoryPullResult,
  SkillFile,
  SkillBranch,
  SkillFocusResponse,
  SkillImportResponse,
  UpdateSkillFilePrimitiveParams,
  UpdateSkillFilePrimitiveResponse,
  UpdateSkillParams,
} from "../types/skills.js";

export class SkillsResource {
  constructor(private readonly http: HTTPClient) {}

  list(params: ListSkillsParams = {}, options?: RequestOptions): Promise<ListSkillsResponse> {
    return this.http.request<ListSkillsResponse>(
      "GET",
      `/v1/skills${buildQuery({
        include_archived: params.include_archived ? "true" : undefined,
        limit: params.limit,
        page_token: params.page_token,
      })}`,
      undefined,
      options,
    );
  }

  create(params: CreateSkillParams = {}, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", "/v1/skills", params, options);
  }

  discover(params: DiscoverSkillsParams = {}, options?: RequestOptions): Promise<ListSkillSummariesResponse> {
    return this.http.request<ListSkillSummariesResponse>("POST", "/v1/skills/discover", params, options);
  }

  focus(params: FocusSkillParams, options?: RequestOptions): Promise<SkillFocusResponse> {
    return this.http.request<SkillFocusResponse>("POST", "/v1/skills/focus", params, options);
  }

  createDev(params: CreateSkillDevParams, options?: RequestOptions): Promise<CreateSkillDevResponse> {
    return this.http.request<CreateSkillDevResponse>("POST", "/v1/skills/create_dev", params, options);
  }

  updateFile(params: UpdateSkillFilePrimitiveParams, options?: RequestOptions): Promise<UpdateSkillFilePrimitiveResponse> {
    return this.http.request<UpdateSkillFilePrimitiveResponse>("POST", "/v1/skills/update_file", params, options);
  }

  retrieve(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("GET", `/v1/skills/${encodeURIComponent(skillID)}`, undefined, options);
  }

  update(skillID: string, params: UpdateSkillParams, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("PATCH", `/v1/skills/${encodeURIComponent(skillID)}`, params, options);
  }

  archive(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", `/v1/skills/${encodeURIComponent(skillID)}/archive`, {}, options);
  }

  delete(skillID: string, options?: RequestOptions): Promise<{ deleted: boolean }> {
    return this.http.request<{ deleted: boolean }>("DELETE", `/v1/skills/${encodeURIComponent(skillID)}`, undefined, options);
  }

  acceptDev(skillID: string, params: { strategy?: SkillAcceptStrategy } = {}, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/accept_dev${buildQuery({ strategy: params.strategy })}`,
      {},
      options,
    );
  }

  discardDev(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", `/v1/skills/${encodeURIComponent(skillID)}/discard_dev`, {}, options);
  }

  listFiles(skillID: string, params: ListSkillFilesParams = {}, options?: RequestOptions): Promise<ListSkillFilesResponse> {
    return this.http.request<ListSkillFilesResponse>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files${buildQuery({
        path: params.path,
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        limit: params.limit,
        page_token: params.page_token,
      })}`,
      undefined,
      options,
    );
  }

  readFile(skillID: string, path: string, params: { branch?: SkillBranch; fallback_to_main?: boolean; max_bytes?: number } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        max_bytes: params.max_bytes,
      })}`,
      undefined,
      options,
    );
  }

  writeFile(skillID: string, path: string, content: string | ArrayBuffer | Blob, params: { branch?: SkillBranch } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.requestRaw<SkillFile>(
      "PUT",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({ branch: params.branch })}`,
      content,
      options,
    );
  }

  deleteFile(skillID: string, path: string, params: { branch?: SkillBranch } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "DELETE",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({ branch: params.branch })}`,
      undefined,
      options,
    );
  }

  exportArchive(skillID: string, params: SkillArchiveParams = {}, options?: RequestOptions): Promise<SkillArchive> {
    const archivePath = normalizeArchivePath(params.path);
    return this.http
      .requestBinary(
        "GET",
        `/v1/skills/${encodeURIComponent(skillID)}/export${buildQuery({
          path: archivePath || undefined,
          branch: params.branch,
          fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        })}`,
        options,
      )
      .then(({ body, headers }) => ({
        path: archivePath,
        content: body,
        content_type: headers.get("Content-Type") ?? undefined,
      }));
  }

  importArchive(
    skillID: string,
    archive: ArrayBuffer | Blob,
    params: ImportSkillArchiveParams = {},
    options?: RequestOptions,
  ): Promise<SkillImportResponse> {
    return this.http.requestRaw<SkillImportResponse>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/import${buildQuery({
        path: normalizeArchivePath(params.path) || undefined,
        branch: params.branch,
        replace: params.replace ? "true" : undefined,
        strip_top_level_dir: params.strip_top_level_dir === false ? "false" : undefined,
      })}`,
      archive,
      options,
    );
  }

  diff(skillID: string, params: SkillBranchDiffParams = {}, options?: RequestOptions): Promise<SkillBranchDiff> {
    return this.http.request<SkillBranchDiff>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/diff${buildQuery({
        path: normalizeArchivePath(params.path) || undefined,
        max_file_chars: params.max_file_chars,
        include_unchanged: params.include_unchanged ? "true" : undefined,
      })}`,
      undefined,
      options,
    );
  }

  async pushDirectory(
    skillID: string,
    rootDir: string,
    params: PushSkillDirectoryParams = {},
    options?: RequestOptions,
  ): Promise<SkillImportResponse> {
    const archive = await zipDirectory(rootDir);
    return this.importArchive(skillID, archive, params, options);
  }

  async pullDirectory(
    skillID: string,
    targetDir: string,
    params: PullSkillDirectoryParams = {},
    options?: RequestOptions,
  ): Promise<SkillDirectoryPullResult> {
    const archive = await this.exportArchive(skillID, params, options);
    return extractStoredZipToDirectory(archive.content, targetDir, { replace: params.replace === true });
  }

}

function skillPath(path: string): string {
  return path
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
}

function normalizeArchivePath(path?: string): string {
  return (path ?? "").trim().replace(/^\/+|\/+$/g, "");
}

async function zipDirectory(rootDir: string): Promise<ArrayBuffer> {
  const fs = await import("node:fs/promises");
  const path = await import("node:path");
  const root = path.resolve(rootDir);
  const files = await walkArchiveFiles(fs, path, root, root);
  const entries: ZipSourceEntry[] = [];
  for (const rel of files.sort()) {
    const content = await fs.readFile(path.join(root, rel));
    entries.push({ path: rel, data: new Uint8Array(content.buffer, content.byteOffset, content.byteLength) });
  }
  return createStoredZip(entries);
}

async function walkArchiveFiles(
  fs: typeof import("node:fs/promises"),
  path: typeof import("node:path"),
  root: string,
  dir: string,
): Promise<string[]> {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const out: string[] = [];
  for (const entry of entries) {
    if (entry.name === ".git" || entry.name === "__pycache__" || entry.name === "node_modules") {
      continue;
    }
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...await walkArchiveFiles(fs, path, root, full));
    } else if (entry.isFile()) {
      out.push(path.relative(root, full).split(path.sep).join("/"));
    }
  }
  return out;
}

async function extractStoredZipToDirectory(
  archive: ArrayBuffer,
  targetDir: string,
  opts: { replace: boolean },
): Promise<SkillDirectoryPullResult> {
  const fs = await import("node:fs/promises");
  const path = await import("node:path");
  const root = path.resolve(targetDir);
  if (opts.replace) {
    await fs.rm(root, { recursive: true, force: true });
  }
  await fs.mkdir(root, { recursive: true });
  let fileCount = 0;
  let byteCount = 0;
  for (const entry of parseStoredZip(archive)) {
    const dest = path.resolve(root, entry.path);
    const rel = path.relative(root, dest);
    if (rel.startsWith("..") || path.isAbsolute(rel)) {
      throw new Error(`archive entry escapes target directory: ${entry.path}`);
    }
    await fs.mkdir(path.dirname(dest), { recursive: true });
    await fs.writeFile(dest, entry.data);
    fileCount += 1;
    byteCount += entry.data.byteLength;
  }
  return { path: root, file_count: fileCount, byte_count: byteCount };
}

type ZipSourceEntry = { path: string; data: Uint8Array };

function createStoredZip(entries: ZipSourceEntry[]): ArrayBuffer {
  const encoder = new TextEncoder();
  const localParts: Uint8Array[] = [];
  const centralParts: Uint8Array[] = [];
  let offset = 0;
  for (const entry of entries) {
    const name = encoder.encode(entry.path);
    const crc = crc32(entry.data);
    const local = new Uint8Array(30 + name.length);
    const lv = new DataView(local.buffer);
    lv.setUint32(0, 0x04034b50, true);
    lv.setUint16(4, 20, true);
    lv.setUint16(8, 0, true);
    lv.setUint32(14, crc, true);
    lv.setUint32(18, entry.data.byteLength, true);
    lv.setUint32(22, entry.data.byteLength, true);
    lv.setUint16(26, name.length, true);
    local.set(name, 30);
    localParts.push(local, entry.data);

    const central = new Uint8Array(46 + name.length);
    const cv = new DataView(central.buffer);
    cv.setUint32(0, 0x02014b50, true);
    cv.setUint16(4, 20, true);
    cv.setUint16(6, 20, true);
    cv.setUint32(16, crc, true);
    cv.setUint32(20, entry.data.byteLength, true);
    cv.setUint32(24, entry.data.byteLength, true);
    cv.setUint16(28, name.length, true);
    cv.setUint32(42, offset, true);
    central.set(name, 46);
    centralParts.push(central);

    offset += local.byteLength + entry.data.byteLength;
  }
  const centralOffset = offset;
  const centralSize = centralParts.reduce((sum, part) => sum + part.byteLength, 0);
  const end = new Uint8Array(22);
  const ev = new DataView(end.buffer);
  ev.setUint32(0, 0x06054b50, true);
  ev.setUint16(8, entries.length, true);
  ev.setUint16(10, entries.length, true);
  ev.setUint32(12, centralSize, true);
  ev.setUint32(16, centralOffset, true);
  const zip = concatUint8([...localParts, ...centralParts, end]);
  return zip.buffer.slice(zip.byteOffset, zip.byteOffset + zip.byteLength) as ArrayBuffer;
}

function parseStoredZip(archive: ArrayBuffer): ZipSourceEntry[] {
  const data = new Uint8Array(archive);
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const entries: ZipSourceEntry[] = [];
  let pos = 0;
  const decoder = new TextDecoder();
  while (pos + 4 <= data.byteLength && view.getUint32(pos, true) === 0x04034b50) {
    const method = view.getUint16(pos + 8, true);
    if (method !== 0) {
      throw new Error("Only stored ZIP entries are supported by pullDirectory");
    }
    const compressedSize = view.getUint32(pos + 18, true);
    const fileNameLength = view.getUint16(pos + 26, true);
    const extraLength = view.getUint16(pos + 28, true);
    const nameStart = pos + 30;
    const contentStart = nameStart + fileNameLength + extraLength;
    const contentEnd = contentStart + compressedSize;
    if (contentEnd > data.byteLength) {
      throw new Error("Invalid ZIP archive");
    }
    const name = decoder.decode(data.slice(nameStart, nameStart + fileNameLength)).replace(/\\/g, "/");
    if (name && !name.endsWith("/") && !name.startsWith("__MACOSX/") && !name.split("/").includes("..")) {
      entries.push({ path: name.replace(/^\/+/, ""), data: data.slice(contentStart, contentEnd) });
    }
    pos = contentEnd;
  }
  return entries;
}

function concatUint8(parts: Uint8Array[]): Uint8Array {
  const total = parts.reduce((sum, part) => sum + part.byteLength, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.byteLength;
  }
  return out;
}

function crc32(data: Uint8Array): number {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let i = 0; i < 8; i += 1) {
      crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}
