import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  createLocalShellToolRegistry,
  createLocalWorkdirToolRegistry,
  localShellToolDefinition,
  localWorkdirToolDefinition,
  LocalWorkdir,
} from "../dist/local/index.js";
import {
  functionCallOutputInput,
  runLocalFunctionHandlers,
} from "../dist/index.js";

test("local workdir registry exposes one model-facing primitive", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-"));
  const workdir = new LocalWorkdir(root, { name: "Tools" });
  await workdir.writeText("README.md", "# Tools\nneedle\n");
  const registry = createLocalWorkdirToolRegistry(workdir);

  const definitions = registry.definitions();
  assert.equal(definitions.length, 1);
  assert.equal(definitions[0].type, "function");
  assert.equal(definitions[0].name, "local_workdir");
  assert.ok(definitions[0].parameters.properties.action.enum.includes("grep"));
  assert.ok(definitions[0].parameters.properties.action.enum.includes("apply_edits"));

  const grep = await registry.execute("local_workdir", { action: "grep", pattern: "needle" });
  assert.equal(grep.ok, true);
  assert.equal(grep.action, "grep");
  assert.equal(grep.object, "list");
  assert.equal(grep.matches[0].path, "README.md");
});

test("local workdir driver covers discovery context and sensitivity actions", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-discovery-"));
  const workdir = new LocalWorkdir(root, { name: "Discovery" });
  await workdir.writeText("README.md", "# Discovery\nneedle\n");
  await workdir.writeText(".env", "SECRET=yes\n");
  const registry = createLocalWorkdirToolRegistry(workdir);

  const entries = await registry.execute("local_workdir", {
    action: "list",
    path: ".",
    options: { recursive: true },
  });
  assert.ok(entries.entries.some((entry) => entry.path === "README.md"));

  const entrySearch = await registry.execute("local_workdir", { action: "search", query: "readme" });
  assert.deepEqual(entrySearch.entries.map((entry) => entry.path), ["README.md"]);

  const snapshot = await registry.execute("local_workdir", { action: "snapshot", options: { hash: true } });
  assert.ok(snapshot.files.some((file) => file.path === "README.md" && file.sha256));

  const classification = await registry.execute("local_workdir", { action: "classify_path", path: ".env" });
  assert.equal(classification.sensitivity, "secret");

  const context = await registry.execute("local_workdir", {
    action: "context",
    query: "needle",
    options: {
      include_search: true,
      max_files: 10,
    },
  });
  assert.equal(context.object, "local_context_manifest");
  assert.ok(context.search.matches.length >= 1);
});

test("local workdir apply edits requires approval by default", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-approval-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("notes.txt", "a\nb\n");
  const registry = createLocalWorkdirToolRegistry(workdir);

  const args = {
    action: "apply_edits",
    edits: [{ path: "notes.txt", start_line: 2, end_line: 2, replacement: "B" }],
  };
  const result = await registry.execute("local_workdir", args);

  assert.equal(result.ok, false);
  assert.equal(result.action, "apply_edits");
  assert.equal(result.requires_approval, true);
  assert.ok(result.preview);
  assert.equal(await workdir.readText("notes.txt"), "a\nb\n");
  assert.equal(registry.requiresApproval("local_workdir", args), true);
});

test("local workdir full access applies edits", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-full-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("notes.txt", "a\nb\n");
  const registry = createLocalWorkdirToolRegistry(workdir, { accessMode: "full" });

  const result = await registry.execute("local_workdir", {
    action: "apply_edits",
    edits: [{ path: "notes.txt", start_line: 2, end_line: 2, replacement: "B" }],
  });

  assert.equal(result.ok, true);
  assert.equal(Array.isArray(result.applied), true);
  assert.equal(await readFile(join(root, "notes.txt"), "utf8"), "a\nB\n");
  assert.equal(registry.requiresApproval("local_workdir", { action: "apply_edits" }), false);
});

test("local workdir mutating actions are approval gated unless full access", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-mutate-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("notes.txt", "hello\n");
  const approval = createLocalWorkdirToolRegistry(workdir);

  const writeApproval = await approval.execute("local_workdir", {
    action: "write",
    path: "created.txt",
    content: "created\n",
  });
  assert.equal(writeApproval.requires_approval, true);
  assert.equal(await workdir.files.exists("created.txt"), false);
  assert.equal(approval.requiresApproval("local_workdir", { action: "delete", path: "notes.txt" }), true);

  const full = createLocalWorkdirToolRegistry(workdir, { accessMode: "full" });
  await full.execute("local_workdir", { action: "write", path: "created.txt", content: "created\n" });
  assert.equal(await workdir.readText("created.txt"), "created\n");

  await full.execute("local_workdir", { action: "mkdir", path: "nested" });
  assert.equal(await workdir.files.exists("nested"), true);

  await full.execute("local_workdir", { action: "delete", path: "created.txt" });
  assert.equal(await workdir.files.exists("created.txt"), false);
});

test("local workdir tools work with local function handlers", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-handler-"));
  const workdir = new LocalWorkdir(root);
  await workdir.writeText("notes.txt", "hello\n");
  const registry = createLocalWorkdirToolRegistry(workdir);

  const response = {
    id: "resp_local",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "mock",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "local_workdir",
        call_id: "call_1",
        arguments: JSON.stringify({ action: "read", path: "notes.txt" }),
      },
    ],
  };

  const outputs = await runLocalFunctionHandlers(response, registry.handlers());
  assert.equal(outputs.length, 1);
  assert.deepEqual(outputs[0], functionCallOutputInput("call_1", JSON.parse(outputs[0].output)));
  assert.match(outputs[0].output, /hello/);
});

test("local workdir tool definition can be renamed for host integrations", () => {
  const definition = localWorkdirToolDefinition("workdir_volume");
  assert.equal(definition.name, "workdir_volume");
  assert.ok(definition.description.includes("local workdir"));
});

test("local shell registry exposes one model-facing primitive", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-"));
  const registry = createLocalShellToolRegistry({ cwd: root });

  const definitions = registry.definitions();
  assert.equal(definitions.length, 1);
  assert.equal(definitions[0].type, "function");
  assert.equal(definitions[0].name, "local_shell");
  assert.equal(definitions[0].parameters.properties.command.type, "string");
  assert.match(definitions[0].description, /platform=/);
  assert.match(definitions[0].description, /access_mode=approval/);
  assert.match(definitions[0].description, /not a filesystem sandbox/);

  const result = await registry.execute("local_shell", {
    command: "printf shell-ready",
    description: "Smoke test shell",
  });
  assert.equal(result.ok, false);
  assert.equal(result.requires_approval, true);
  assert.equal(result.command, "printf shell-ready");
  assert.equal(registry.requiresApproval("local_shell"), true);
});

test("local shell full access executes commands in configured cwd", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-full-"));
  const registry = createLocalShellToolRegistry({ cwd: root, accessMode: "full" });

  const result = await registry.execute("local_shell", {
    command: "printf shell-output > result.txt && printf done",
    description: "Write result file",
  });

  assert.equal(result.ok, true);
  assert.equal(result.exit_code, 0);
  assert.match(result.output, /done/);
  assert.equal(await readFile(join(root, "result.txt"), "utf8"), "shell-output");
  assert.equal(registry.requiresApproval("local_shell"), false);
});

test("local shell rejects workdir traversal outside configured cwd", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-contained-"));
  const registry = createLocalShellToolRegistry({ cwd: root, accessMode: "full" });

  await assert.rejects(
    () => registry.execute("local_shell", { command: "pwd", workdir: ".." }),
    /workdir must stay inside/,
  );
});

test("local shell tools work with local function handlers", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-handler-"));
  const registry = createLocalShellToolRegistry({ cwd: root, accessMode: "full" });

  const response = {
    id: "resp_local_shell",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "mock",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "local_shell",
        call_id: "call_1",
        arguments: JSON.stringify({ command: "printf local-shell" }),
      },
    ],
  };

  const outputs = await runLocalFunctionHandlers(response, registry.handlers());
  assert.equal(outputs.length, 1);
  assert.deepEqual(outputs[0], functionCallOutputInput("call_1", JSON.parse(outputs[0].output)));
  assert.match(outputs[0].output, /local-shell/);
});

test("local shell tool definition can be renamed for host integrations", () => {
  const definition = localShellToolDefinition("host_command", {
    accessMode: "full",
    cwd: "/tmp/example",
    platform: "linux",
    shell: "bash",
    timeoutMs: 1000,
    maxOutputBytes: 2048,
  });
  assert.equal(definition.name, "host_command");
  assert.ok(definition.description.includes("local shell"));
  assert.match(definition.description, /platform=linux/);
  assert.match(definition.description, /shell=bash/);
  assert.match(definition.description, /access_mode=full/);
  assert.match(definition.description, /default_timeout_ms=1000/);
  assert.match(definition.description, /max_output_bytes=2048/);
});
