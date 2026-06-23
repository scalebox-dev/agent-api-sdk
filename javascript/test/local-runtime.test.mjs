import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  classifyLocalPathSensitivity,
  createLocalContextPackage,
  createLocalRuntime,
  localAppDirs,
  LocalFileStore,
  LocalIgnoredPathError,
  LocalError,
  LocalWorkdir,
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

test("LocalFileStore skips broken symlinks during recursive scans", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-broken-symlink-"));
  const store = new LocalFileStore(root);
  await store.writeText("README.md", "# Project\nneedle\n");
  await symlink(join(root, "missing-target"), join(root, "SingletonCookie"));

  const files = await store.list(".", { recursive: true });
  assert.deepEqual(files.map((item) => [item.path, item.type]), [
    ["README.md", "file"],
    ["SingletonCookie", "symlink"],
  ]);

  const summary = await store.summarize();
  assert.equal(summary.file_count, 1);

  const grep = await store.grep({ pattern: "needle" });
  assert.deepEqual(grep.matches.map((match) => match.path), ["README.md"]);
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

test("LocalWorkdir applies default ignore rules and scoped workbench operations", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workdir-"));
  const workdir = new LocalWorkdir(root, { name: "Demo", trusted: true });
  await workdir.writeText("src/index.ts", "hello\nneedle\n");
  await workdir.files.writeText("node_modules/pkg/index.js", "needle\n");

  const grep = await workdir.grep({ pattern: "needle" });
  assert.deepEqual(grep.matches.map((match) => match.path), ["src/index.ts"]);
  assert.throws(() => workdir.resolvePath("node_modules/pkg/index.js"), LocalIgnoredPathError);

  const summary = await workdir.summarize();
  assert.equal(summary.file_count, 1);
  assert.equal(workdir.name, "Demo");
  assert.equal(workdir.trusted, true);
});

test("LocalWorkdir loads .gitignore rules", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-gitignore-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText(".gitignore", "ignored-dir/\n*.tmp\n");
  await workdir.files.writeText("ignored-dir/a.txt", "hidden\n");
  await workdir.files.writeText("keep.tmp", "hidden\n");
  await workdir.writeText("src/index.ts", "visible\n");

  await workdir.loadIgnoreFiles();
  const entries = await workdir.listEntries(".", { recursive: true });
  assert.deepEqual(entries.entries.map((entry) => entry.path), [".gitignore", "src", "src/index.ts"]);
});

test("LocalWorkdir previews and applies line edits", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workdir-edit-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("notes.txt", "a\nb\nc\n");

  const preview = await workdir.previewPatchLines("notes.txt", {
    startLine: 2,
    endLine: 2,
    replacement: "B",
  });
  assert.deepEqual(preview.before, ["b"]);
  assert.deepEqual(preview.after, ["B"]);

  await workdir.patchLines("notes.txt", {
    startLine: 2,
    endLine: 2,
    replacement: "B",
  });
  assert.equal(await workdir.readText("notes.txt"), "a\nB\nc\n");
});

test("LocalWorkdir snapshots and diffs local changes", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-workdir-diff-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("a.txt", "a\n");
  await workdir.writeText("b.txt", "b\n");
  const before = await workdir.snapshot();

  await workdir.writeText("a.txt", "changed\n");
  await workdir.deletePath("b.txt");
  await workdir.writeText("c.txt", "c\n");
  const after = await workdir.snapshot();
  const diff = workdir.diff(before, after);

  assert.deepEqual(diff.added.map((file) => file.path), ["c.txt"]);
  assert.deepEqual(diff.deleted.map((file) => file.path), ["b.txt"]);
  assert.deepEqual(diff.modified.map((item) => item.after.path), ["a.txt"]);
});

test("LocalWorkdir edit plans detect conflicts and roll back failed multi-file edits", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-edit-plan-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("a.txt", "a\n");
  await workdir.writeText("b.txt", "b\n");
  const snapshot = await workdir.snapshot();
  const aHash = snapshot.files.find((file) => file.path === "a.txt")?.sha256;

  const plan = await workdir.previewEdits([
    { path: "a.txt", startLine: 1, endLine: 1, replacement: "A", expectedSha256: aHash },
  ]);
  assert.deepEqual(plan.previews[0].before, ["a"]);
  assert.deepEqual(plan.previews[0].after, ["A"]);

  await workdir.writeText("a.txt", "changed\n");
  await assert.rejects(
    () => workdir.applyEdits([{ path: "a.txt", startLine: 1, replacement: "A", expectedSha256: aHash }]),
    (error) => error instanceof LocalError && error.code === "local_edit_conflict",
  );

  await workdir.writeText("a.txt", "a\n");
  await assert.rejects(
    () =>
      workdir.applyEdits([
        { path: "a.txt", startLine: 1, endLine: 1, replacement: "A" },
        { path: "b.txt", startLine: 99, endLine: 99, replacement: "B" },
      ]),
    /invalid line range/,
  );
  assert.equal(await workdir.readText("a.txt"), "a\n");
  assert.equal(await workdir.readText("b.txt"), "b\n");
});

test("local path sensitivity classification identifies likely secrets", () => {
  assert.equal(classifyLocalPathSensitivity(".env").sensitivity, "secret");
  assert.equal(classifyLocalPathSensitivity("config/service-token.json").sensitivity, "sensitive");
  assert.equal(classifyLocalPathSensitivity("src/index.ts").sensitivity, "normal");
});

test("LocalRuntime opens workdirs", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-runtime-workdir-"));
  const runtime = createLocalRuntime({ appName: "agent-studio", baseDir: root });
  const workdir = runtime.workdir(join(root, "project"), { name: "Project" });
  await workdir.ensure();
  await workdir.writeText("README.md", "# Project\n");

  assert.equal(workdir.name, "Project");
  assert.equal((await workdir.list()).length, 1);
  assert.equal(runtime.workdirs.open(join(root, "other")).name, "other");
});

test("LocalWorkdir watcher returns a close handle", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-watch-"));
  const workdir = new LocalWorkdir(root);
  await workdir.ensure();

  const watcher = workdir.watch(() => {});
  watcher.close();
});

test("Local context packages budget workdir files for agent handoff", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-context-"));
  const workdir = new LocalWorkdir(root, { name: "Context Demo" });
  await workdir.writeText("README.md", "# Demo\nneedle\n");
  await workdir.writeText("src/index.ts", "console.log('needle');\n");
  await workdir.writeText(".env", "TOKEN=secret\n");

  const manifest = await createLocalContextPackage(workdir, {
    query: "needle",
    includeSearch: true,
    maxFiles: 10,
    maxBytes: 1024,
  });

  assert.equal(manifest.object, "local_context_manifest");
  assert.equal(manifest.workdir_name, "Context Demo");
  assert.equal(manifest.file_count, 3);
  assert.ok(manifest.summary);
  assert.ok(manifest.search?.matches.length >= 2);

  const envFile = manifest.files.find((file) => file.path === ".env");
  assert.equal(envFile?.sensitivity, "secret");
  assert.equal(envFile?.omitted_reason, "secret_path");
  assert.equal(envFile?.content, undefined);

  const readme = manifest.files.find((file) => file.path === "README.md");
  assert.equal(readme?.encoding, "text");
  assert.match(readme?.content ?? "", /needle/);
  assert.match(readme?.sha256 ?? "", /^[a-f0-9]{64}$/);
});

test("Local context packages can include explicit secret content", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-context-secret-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText(".env", "TOKEN=secret\n");

  const manifest = await createLocalContextPackage(workdir, {
    includeSecrets: true,
    includeSummary: false,
  });

  const envFile = manifest.files.find((file) => file.path === ".env");
  assert.equal(envFile?.sensitivity, "secret");
  assert.equal(envFile?.omitted_reason, undefined);
  assert.match(envFile?.content ?? "", /TOKEN=secret/);
});
