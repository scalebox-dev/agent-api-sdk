import type { LocalSkillDescriptor } from "./types/skills.js";
import { functionCallOutputInput, pendingFunctionCalls } from "./local-functions.js";
import type { FunctionCallOutputInput } from "./types/input.js";
import type { AgentResponse, FunctionCallOutputItem } from "./types/responses.js";

const DEFAULT_FOCUS_MANIFEST_CHARS = 16000;
const DEFAULT_FOCUS_FILE_CHARS = 12000;

export interface LocalSkillDirectoryOptions {
  id?: string;
  name?: string;
  description?: string;
  max_manifest_chars?: number;
  metadata?: Record<string, unknown>;
}

export async function localSkillFromDirectory(
  rootDir: string,
  options: LocalSkillDirectoryOptions = {},
): Promise<LocalSkillDescriptor> {
  const fs = await import("node:fs/promises");
  const path = await import("node:path");
  const crypto = await import("node:crypto");
  const root = path.resolve(rootDir);
  const files = await walkFiles(fs, path, root, root);
  const hash = crypto.createHash("sha256");
  for (const rel of files.sort()) {
    hash.update(rel);
    hash.update("\0");
    hash.update(await fs.readFile(path.join(root, rel)));
    hash.update("\0");
  }
  const digest = "sha256:" + hash.digest("hex");
  const base = path.basename(root);
  const localSkillID = options.id ?? base;
  const { manifest, manifestTruncated } = await readLocalManifest(fs, path, root, options.max_manifest_chars ?? DEFAULT_FOCUS_MANIFEST_CHARS);
  return {
    local_skill_id: localSkillID,
    skill_ref: skillRefForLocal(localSkillID, digest),
    name: options.name ?? base,
    description: options.description,
    root_hint: root,
    digest,
    manifest,
    manifest_truncated: manifestTruncated,
    metadata: options.metadata,
  };
}

export function pendingLocalSkillCalls(response: AgentResponse): FunctionCallOutputItem[] {
  return pendingFunctionCalls(response).filter((call) => call.name === "skill_focus");
}

export async function runLocalSkillHandlers(
  response: AgentResponse,
  localSkills: LocalSkillDescriptor[],
): Promise<FunctionCallOutputInput[]> {
  const byRef = await localSkillsByRef(localSkills);
  const outputs: FunctionCallOutputInput[] = [];
  for (const call of pendingLocalSkillCalls(response)) {
    const args = call.arguments ? JSON.parse(call.arguments) as Record<string, unknown> : {};
    const payload = await focusLocalSkills(args, byRef);
    outputs.push(functionCallOutputInput(call.call_id, payload));
  }
  return outputs;
}

async function focusLocalSkills(
  args: Record<string, unknown>,
  byRef: Map<string, LocalSkillDescriptor>,
): Promise<Record<string, unknown>> {
  const maxManifestChars = manifestCharLimit(args);
  const maxFileChars = fileCharLimit(args);
  const items = Array.isArray(args.skills) ? args.skills : [];
  if (!Array.isArray(args.skills)) {
    return { data: [{ ok: false, error: { code: "invalid_skill_focus", message: "skills must be an array" } }] };
  }
  const data: Record<string, unknown>[] = [];
  for (const raw of items) {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
      data.push({ ok: false, error: { code: "invalid_skill_focus", message: "skill item must be an object" } });
      continue;
    }
    const item = raw as Record<string, unknown>;
    const skillRef = String(item.skill_ref || "").trim();
    const result: Record<string, unknown> = { ok: false, skill_ref: skillRef, branch: "main" };
    const descriptor = byRef.get(skillRef);
    if (!descriptor) {
      result.error = { code: "skill_ref_not_found", message: "skill_ref was not registered with the SDK" };
      data.push(result);
      continue;
    }
    try {
      result.skill = await focusedLocalSkill(descriptor, maxManifestChars, pathsArg(item), maxFileChars, includeManifest(item));
      result.ok = true;
    } catch (error) {
      result.error = { code: "local_skill_focus_failed", message: error instanceof Error ? error.message : String(error) };
    }
    data.push(result);
  }
  return { data };
}

async function focusedLocalSkill(
  descriptor: LocalSkillDescriptor,
  maxManifestChars: number,
  paths: string[],
  maxFileChars: number,
  includeManifest: boolean,
): Promise<Record<string, unknown>> {
  const fs = await import("node:fs/promises");
  const path = await import("node:path");
  const { TextDecoder } = await import("node:util");
  const root = path.resolve(descriptor.root_hint || ".");
  const stat = await fs.stat(root);
  if (!stat.isDirectory()) {
    throw new Error(`local skill root is not a directory: ${root}`);
  }
  let manifest = "";
  let manifestTruncated = false;
  if (includeManifest) {
    try {
      const manifestBytes = await fs.readFile(path.join(root, "SKILL.md"));
      manifest = new TextDecoder("utf-8", { fatal: true }).decode(manifestBytes);
      const manifestChars = Array.from(manifest);
      if (maxManifestChars > 0 && manifestChars.length > maxManifestChars) {
        manifest = manifestChars.slice(0, maxManifestChars).join("");
        manifestTruncated = true;
      }
    } catch (error: any) {
      if (error?.code !== "ENOENT") {
        throw error;
      }
    }
  }
  const dirents = await fs.readdir(root, { withFileTypes: true });
  const entries = await Promise.all(
    dirents
      .filter((entry) => ![".git", "__pycache__", "node_modules"].includes(entry.name))
      .sort((a, b) => a.name.localeCompare(b.name))
      .map(async (entry) => {
        const full = path.join(root, entry.name);
        const entryStat = await fs.stat(full);
        return {
          path: entry.name,
          is_dir: entry.isDirectory(),
          size: entry.isDirectory() ? 0 : entryStat.size,
          modified_at: Math.floor(entryStat.mtimeMs / 1000),
        };
      }),
  );
  return {
    skill_ref: await descriptorSkillRef(descriptor),
    name: descriptor.name || "",
    description: descriptor.description || "",
    branch: "SKILL_BRANCH_MAIN",
    digest: descriptor.digest || "",
    manifest,
    manifest_truncated: manifestTruncated,
    entries,
    files: await Promise.all(paths.map((relPath) => focusedLocalFile(fs, path, root, relPath, maxFileChars))),
  };
}

function manifestCharLimit(args: Record<string, unknown>): number {
  const raw = args.max_manifest_chars ?? DEFAULT_FOCUS_MANIFEST_CHARS;
  const value = Number(raw);
  return Number.isFinite(value) ? value : DEFAULT_FOCUS_MANIFEST_CHARS;
}

function fileCharLimit(args: Record<string, unknown>): number {
  const raw = args.max_file_chars ?? DEFAULT_FOCUS_FILE_CHARS;
  const value = Number(raw);
  return Number.isFinite(value) ? value : DEFAULT_FOCUS_FILE_CHARS;
}

function pathsArg(item: Record<string, unknown>): string[] {
  if (!Array.isArray(item.paths)) {
    return [];
  }
  return item.paths
    .filter((value): value is string => typeof value === "string")
    .map((value) => value.trim())
    .filter(Boolean);
}

function includeManifest(item: Record<string, unknown>): boolean {
  return typeof item.include_manifest === "boolean" ? item.include_manifest : true;
}

async function focusedLocalFile(
  fs: typeof import("node:fs/promises"),
  path: typeof import("node:path"),
  root: string,
  relPath: string,
  maxFileChars: number,
): Promise<Record<string, unknown>> {
  const base = { path: relPath, branch: "SKILL_BRANCH_MAIN", content: "", truncated: false, size: 0 };
  const { TextDecoder } = await import("node:util");
  try {
    const full = path.resolve(root, relPath);
    const relative = path.relative(root, full);
    if (relative.startsWith("..") || path.isAbsolute(relative)) {
      return { ...base, error: { type: "skill_error", code: "invalid_skill_file_path", message: "path must stay inside the local skill root" } };
    }
    const stat = await fs.stat(full);
    if (!stat.isFile()) {
      return { ...base, error: { type: "skill_error", code: "skill_file_not_found", message: "skill file was not found" } };
    }
    const raw = await fs.readFile(full);
    let content = new TextDecoder("utf-8", { fatal: true }).decode(raw);
    let truncated = false;
    const chars = Array.from(content);
    if (maxFileChars > 0 && chars.length > maxFileChars) {
      content = chars.slice(0, maxFileChars).join("");
      truncated = true;
    }
    return { ...base, content, truncated, size: stat.size };
  } catch (error: any) {
    if (error?.code === "ENOENT") {
      return { ...base, error: { type: "skill_error", code: "skill_file_not_found", message: "skill file was not found" } };
    }
    if (error instanceof TypeError) {
      return { ...base, error: { type: "skill_error", code: "invalid_skill_file_utf8", message: "skill file must be valid UTF-8" } };
    }
    return { ...base, error: { type: "skill_error", code: "skill_file_read_failed", message: error instanceof Error ? error.message : String(error) } };
  }
}

async function walkFiles(fs: any, path: any, root: string, dir: string): Promise<string[]> {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const out: string[] = [];
  for (const entry of entries) {
    if (entry.name === ".git" || entry.name === "node_modules") {
      continue;
    }
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...await walkFiles(fs, path, root, full));
      continue;
    }
    if (entry.isFile()) {
      out.push(path.relative(root, full).split(path.sep).join("/"));
    }
  }
  return out;
}

async function localSkillsByRef(localSkills: LocalSkillDescriptor[]): Promise<Map<string, LocalSkillDescriptor>> {
  const out = new Map<string, LocalSkillDescriptor>();
  for (const skill of localSkills) {
    out.set(await descriptorSkillRef(skill), skill);
  }
  return out;
}

async function descriptorSkillRef(descriptor: LocalSkillDescriptor): Promise<string> {
  const existing = String(descriptor.skill_ref || "").trim();
  if (existing) {
    return existing;
  }
  return skillRefForLocal(descriptor.local_skill_id || "", descriptor.digest || "");
}

function skillRefForLocal(
  localSkillID: string,
  digest: string,
): string {
  const slug = skillRefSlug(localSkillID) || "local-skill";
  const digestPart = skillRefDigestPart(digest) || "unknown";
  return `local::${slug}@${digestPart}::main`;
}

function skillRefSlug(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .replace(/::/g, "-")
    .replace(/@/g, "-")
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^[-_]+|[-_]+$/g, "")
    .slice(0, 64)
    .replace(/^[-_]+|[-_]+$/g, "");
}

function skillRefDigestPart(raw: string): string {
  const digest = raw.trim().toLowerCase().split(":").pop() || "";
  return digest.replace(/[^a-z0-9_-]+/g, "").slice(0, 16);
}

async function readLocalManifest(
  fs: typeof import("node:fs/promises"),
  path: typeof import("node:path"),
  root: string,
  maxManifestChars: number,
): Promise<{ manifest: string; manifestTruncated: boolean }> {
  const { TextDecoder } = await import("node:util");
  try {
    const raw = await fs.readFile(path.join(root, "SKILL.md"));
    let manifest = new TextDecoder("utf-8", { fatal: true }).decode(raw);
    let manifestTruncated = false;
    const chars = Array.from(manifest);
    if (maxManifestChars > 0 && chars.length > maxManifestChars) {
      manifest = chars.slice(0, maxManifestChars).join("");
      manifestTruncated = true;
    }
    return { manifest, manifestTruncated };
  } catch (error: any) {
    if (error?.code === "ENOENT") {
      return { manifest: "", manifestTruncated: false };
    }
    throw error;
  }
}
