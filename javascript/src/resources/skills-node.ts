import { SkillsResource } from "./skills.js";
import type { RequestOptions } from "../types/common.js";
import type {
  PullSkillDirectoryParams,
  PushSkillDirectoryParams,
  SkillDirectoryPullResult,
  SkillImportResponse,
} from "../types/skills.js";

export class NodeSkillsResource extends SkillsResource {
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
