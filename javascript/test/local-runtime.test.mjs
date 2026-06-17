import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  createLocalRuntime,
  localAppDirs,
  LocalFileStore,
  LocalWorkspace,
} from "../dist/local/index.js";

test("localAppDirs follows XDG directories on Linux", () => {
  const dirs = localAppDirs({
    appName: "Agent Studio",
    platform: "linux",
    env: {
      HOME: "/home/dev",
      XDG_DATA_HOME: "/xdg/data",
      XDG_CONFIG_HOME: "/xdg/config",
      XDG_CACHE_HOME: "/xdg/cache",
      XDG_STATE_HOME: "/xdg/state",
    },
  });

  assert.equal(dirs.data, "/xdg/data/agent-studio");
  assert.equal(dirs.config, "/xdg/config/agent-studio");
  assert.equal(dirs.cache, "/xdg/cache/agent-studio");
  assert.equal(dirs.logs, "/xdg/state/agent-studio/logs");
});

test("LocalFileStore rejects traversal and absolute paths", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-store-"));
  const store = new LocalFileStore(root);

  assert.throws(() => store.resolvePath("../outside.txt"), /inside the store root|relative/);
  assert.throws(() => store.resolvePath("/tmp/outside.txt"), /relative/);

  await store.writeText("notes/hello.txt", "hello");
  assert.equal(await store.readText("notes/hello.txt"), "hello");
});

test("local runtime reads and writes config JSON", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-runtime-"));
  const runtime = createLocalRuntime({ appName: "agent-studio", baseDir: root });
  await runtime.ensure();

  await runtime.config.write("settings.json", { theme: "dark" });
  assert.deepEqual(await runtime.config.read("settings.json"), { theme: "dark" });

  await runtime.config.set("settings.json", "apiBaseURL", "https://agent.test");
  assert.equal(await runtime.config.get("settings.json", "apiBaseURL"), "https://agent.test");

  const raw = await readFile(join(root, "config", "settings.json"), "utf8");
  assert.match(raw, /"theme": "dark"/);
});

test("LocalFileStore lists files recursively and preserves unrelated metadata", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-list-"));
  const store = new LocalFileStore(root);
  await store.writeJSON("cache/models.json", [{ id: "m" }]);
  await store.writeText("logs/app.log", "ok");

  const files = await store.list(".", { recursive: true });
  assert.deepEqual(files.map((item) => item.path), ["cache/models.json", "logs/app.log"]);
  assert.equal(files[0].type, "file");
});

test("local runtime discovers local skills", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skills-"));
  const runtime = createLocalRuntime({ appName: "agent-studio", baseDir: root });
  await mkdir(join(runtime.dirs.data, "skills", "release-triage"), { recursive: true });
  await writeFile(join(runtime.dirs.data, "skills", "release-triage", "SKILL.md"), "# Release Triage\n");

  const skills = await runtime.skills.discover();
  assert.equal(skills.length, 1);
  assert.equal(skills[0].local_skill_id, "release-triage");
  assert.equal(skills[0].name, "release-triage");
  assert.match(skills[0].skill_ref, /^local::release-triage@/);
});

test("LocalFileStore supports workbench-style entry search and file delivery", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workbench-"));
  const store = new LocalFileStore(root);
  await store.writeText("notes/hello.md", "# Hello\nneedle one\n");
  await store.writeBytes("assets/blob.bin", new Uint8Array([0, 1, 2, 3]));

  const entries = await store.listEntries(".", { recursive: true });
  assert.ok(entries.entries.some((entry) => entry.path === "notes/hello.md" && !entry.is_dir));

  const search = await store.searchEntries({ query: "hello" });
  assert.deepEqual(search.entries.map((entry) => entry.path), ["notes/hello.md"]);

  const text = await store.readFile("notes/hello.md");
  assert.equal(text.encoding, "text");
  assert.match(text.content, /needle one/);

  const binary = await store.readFile("assets/blob.bin");
  assert.equal(binary.encoding, "base64");
  assert.equal(binary.content_base64, "AAECAw==");
});

test("LocalFileStore reads and patches line ranges", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-lines-"));
  const store = new LocalFileStore(root);
  await store.writeText("notes/hello.txt", "a\nb\nc\n");

  const lines = await store.readLines("notes/hello.txt", { startLine: 2, endLine: 3 });
  assert.deepEqual(lines.lines, ["b", "c"]);
  assert.equal(lines.total_lines, 3);
  assert.equal(lines.end_line, 3);

  const patch = await store.patchLines("notes/hello.txt", {
    startLine: 2,
    endLine: 2,
    replacement: "B\nB2",
  });
  assert.equal(patch.total_lines, 4);
  assert.equal(await store.readText("notes/hello.txt"), "a\nB\nB2\nc\n");
});

test("LocalFileStore greps local text files", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-grep-"));
  const store = new LocalFileStore(root);
  await store.writeText("src/a.ts", "needle\nnope\n");
  await store.writeText("src/b.ts", "again needle\n");
  await store.writeBytes("src/blob.bin", new Uint8Array([0, 110, 101, 101, 100, 108, 101]));

  const result = await store.grep({ pattern: "needle", path: "src", limit: 10 });
  assert.equal(result.object, "list");
  assert.equal(result.files_scanned, 2);
  assert.deepEqual(
    result.matches.map((match) => [match.path, match.line_number, match.line]),
    [
      ["src/a.ts", 1, "needle"],
      ["src/b.ts", 1, "again needle"],
    ],
  );
});

test("LocalFileStore scoped operations return store-root-relative paths", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-root-relative-"));
  const store = new LocalFileStore(root);
  await store.writeText("src/nested/a.ts", "needle\n");

  const entries = await store.listEntries("src", { recursive: true });
  assert.deepEqual(entries.entries.map((entry) => entry.path), ["src/nested", "src/nested/a.ts"]);

  const grep = await store.grep({ pattern: "needle", path: "src" });
  assert.deepEqual(grep.matches.map((match) => match.path), ["src/nested/a.ts"]);
});

test("LocalFileStore summarizes local workdirs", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-summary-"));
  const store = new LocalFileStore(root);
  await store.writeText("README.md", "# Project\n");
  await store.writeText("src/index.ts", "console.log('hello');\n");

  const summary = await store.summarize({ maxPreviews: 2 });
  assert.equal(summary.file_count, 2);
  assert.ok(summary.total_bytes > 0);
  assert.ok(summary.top_paths_by_size.some((item) => item.includes("README.md")));
  assert.ok(summary.text_previews.some((preview) => preview.path === "README.md"));
  assert.equal(summary.summary_path, "");
});

test("LocalWorkspace applies default ignore rules and scoped workbench operations", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workspace-"));
  const workspace = new LocalWorkspace(root, { name: "Demo", trusted: true });
  await workspace.writeText("src/index.ts", "hello\nneedle\n");
  await workspace.files.writeText("node_modules/pkg/index.js", "needle\n");

  const grep = await workspace.grep({ pattern: "needle" });
  assert.deepEqual(grep.matches.map((match) => match.path), ["src/index.ts"]);
  assert.throws(() => workspace.resolvePath("node_modules/pkg/index.js"), /ignored/);

  const summary = await workspace.summarize();
  assert.equal(summary.file_count, 1);
  assert.equal(workspace.name, "Demo");
  assert.equal(workspace.trusted, true);
});

test("LocalWorkspace previews and applies line edits", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workspace-edit-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("notes.txt", "a\nb\nc\n");

  const preview = await workspace.previewPatchLines("notes.txt", {
    startLine: 2,
    endLine: 2,
    replacement: "B",
  });
  assert.deepEqual(preview.before, ["b"]);
  assert.deepEqual(preview.after, ["B"]);

  await workspace.patchLines("notes.txt", {
    startLine: 2,
    endLine: 2,
    replacement: "B",
  });
  assert.equal(await workspace.readText("notes.txt"), "a\nB\nc\n");
});

test("LocalWorkspace snapshots and diffs local changes", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workspace-diff-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("a.txt", "a\n");
  await workspace.writeText("b.txt", "b\n");
  const before = await workspace.snapshot();

  await workspace.writeText("a.txt", "changed\n");
  await workspace.deletePath("b.txt");
  await workspace.writeText("c.txt", "c\n");
  const after = await workspace.snapshot();
  const diff = workspace.diff(before, after);

  assert.deepEqual(diff.added.map((file) => file.path), ["c.txt"]);
  assert.deepEqual(diff.deleted.map((file) => file.path), ["b.txt"]);
  assert.deepEqual(diff.modified.map((item) => item.after.path), ["a.txt"]);
});

test("LocalRuntime opens workspaces", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-runtime-workspace-"));
  const runtime = createLocalRuntime({ appName: "agent-studio", baseDir: root });
  const workspace = runtime.workspace(join(root, "project"), { name: "Project" });
  await workspace.ensure();
  await workspace.writeText("README.md", "# Project\n");

  assert.equal(workspace.name, "Project");
  assert.equal((await workspace.list()).length, 1);
  assert.equal(runtime.workspaces.open(join(root, "other")).name, "other");
});

test("LocalWorkspace watcher returns a close handle", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-watch-"));
  const workspace = new LocalWorkspace(root);
  await workspace.ensure();

  const watcher = workspace.watch(() => {});
  watcher.close();
});
