import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  createLocalWorkspaceToolRegistry,
  localWorkspaceToolDefinition,
  LocalWorkspace,
} from "../dist/local/index.js";
import {
  functionCallOutputInput,
  runLocalFunctionHandlers,
} from "../dist/index.js";

test("local workspace registry exposes one model-facing primitive", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-"));
  const workspace = new LocalWorkspace(root, { name: "Tools" });
  await workspace.writeText("README.md", "# Tools\nneedle\n");
  const registry = createLocalWorkspaceToolRegistry(workspace);

  const definitions = registry.definitions();
  assert.equal(definitions.length, 1);
  assert.equal(definitions[0].type, "function");
  assert.equal(definitions[0].name, "local_workspace");
  assert.ok(definitions[0].parameters.properties.action.enum.includes("grep"));
  assert.ok(definitions[0].parameters.properties.action.enum.includes("apply_edits"));

  const grep = await registry.execute("local_workspace", { action: "grep", pattern: "needle" });
  assert.equal(grep.ok, true);
  assert.equal(grep.action, "grep");
  assert.equal(grep.object, "list");
  assert.equal(grep.matches[0].path, "README.md");
});

test("local workspace driver covers discovery context and sensitivity actions", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-discovery-"));
  const workspace = new LocalWorkspace(root, { name: "Discovery" });
  await workspace.writeText("README.md", "# Discovery\nneedle\n");
  await workspace.writeText(".env", "SECRET=yes\n");
  const registry = createLocalWorkspaceToolRegistry(workspace);

  const entries = await registry.execute("local_workspace", {
    action: "list",
    path: ".",
    options: { recursive: true },
  });
  assert.ok(entries.entries.some((entry) => entry.path === "README.md"));

  const entrySearch = await registry.execute("local_workspace", { action: "search", query: "readme" });
  assert.deepEqual(entrySearch.entries.map((entry) => entry.path), ["README.md"]);

  const snapshot = await registry.execute("local_workspace", { action: "snapshot", options: { hash: true } });
  assert.ok(snapshot.files.some((file) => file.path === "README.md" && file.sha256));

  const classification = await registry.execute("local_workspace", { action: "classify_path", path: ".env" });
  assert.equal(classification.sensitivity, "secret");

  const context = await registry.execute("local_workspace", {
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

test("local workspace apply edits requires approval by default", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-approval-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("notes.txt", "a\nb\n");
  const registry = createLocalWorkspaceToolRegistry(workspace);

  const args = {
    action: "apply_edits",
    edits: [{ path: "notes.txt", start_line: 2, end_line: 2, replacement: "B" }],
  };
  const result = await registry.execute("local_workspace", args);

  assert.equal(result.ok, false);
  assert.equal(result.action, "apply_edits");
  assert.equal(result.requires_approval, true);
  assert.ok(result.preview);
  assert.equal(await workspace.readText("notes.txt"), "a\nb\n");
  assert.equal(registry.requiresApproval("local_workspace", args), true);
});

test("local workspace full access applies edits", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-full-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("notes.txt", "a\nb\n");
  const registry = createLocalWorkspaceToolRegistry(workspace, { accessMode: "full" });

  const result = await registry.execute("local_workspace", {
    action: "apply_edits",
    edits: [{ path: "notes.txt", start_line: 2, end_line: 2, replacement: "B" }],
  });

  assert.equal(result.ok, true);
  assert.equal(Array.isArray(result.applied), true);
  assert.equal(await readFile(join(root, "notes.txt"), "utf8"), "a\nB\n");
  assert.equal(registry.requiresApproval("local_workspace", { action: "apply_edits" }), false);
});

test("local workspace mutating actions are approval gated unless full access", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-mutate-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("notes.txt", "hello\n");
  const approval = createLocalWorkspaceToolRegistry(workspace);

  const writeApproval = await approval.execute("local_workspace", {
    action: "write",
    path: "created.txt",
    content: "created\n",
  });
  assert.equal(writeApproval.requires_approval, true);
  assert.equal(await workspace.files.exists("created.txt"), false);
  assert.equal(approval.requiresApproval("local_workspace", { action: "delete", path: "notes.txt" }), true);

  const full = createLocalWorkspaceToolRegistry(workspace, { accessMode: "full" });
  await full.execute("local_workspace", { action: "write", path: "created.txt", content: "created\n" });
  assert.equal(await workspace.readText("created.txt"), "created\n");

  await full.execute("local_workspace", { action: "mkdir", path: "nested" });
  assert.equal(await workspace.files.exists("nested"), true);

  await full.execute("local_workspace", { action: "delete", path: "created.txt" });
  assert.equal(await workspace.files.exists("created.txt"), false);
});

test("local workspace tools work with local function handlers", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-tools-handler-"));
  const workspace = new LocalWorkspace(root);
  await workspace.writeText("notes.txt", "hello\n");
  const registry = createLocalWorkspaceToolRegistry(workspace);

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
        name: "local_workspace",
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

test("local workspace tool definition can be renamed for host integrations", () => {
  const definition = localWorkspaceToolDefinition("workspace_volume");
  assert.equal(definition.name, "workspace_volume");
  assert.ok(definition.description.includes("local workspace"));
});
