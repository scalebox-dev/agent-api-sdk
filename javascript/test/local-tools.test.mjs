import assert from "node:assert/strict";
import { chmod, mkdtemp, readFile, writeFile } from "node:fs/promises";
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

  const fileGrep = await registry.execute("local_workdir", { action: "grep", path: "README.md", pattern: "needle" });
  assert.equal(fileGrep.ok, true);
  assert.equal(fileGrep.matches.length, 1);
  assert.equal(fileGrep.matches[0].path, "README.md");

  const missing = await registry.execute("local_workdir", { action: "grep", path: "missing.txt", pattern: "needle" });
  assert.equal(missing.ok, false);
  assert.equal(missing.action, "grep");
  assert.match(missing.error, /no such file|cannot find|ENOENT/i);
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
  assert.deepEqual(result.changed_files, ["notes.txt"]);
  assert.equal(result.edit_count, 1);
  assert.equal("backups" in result, false);
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
  assert.equal(result.shell_isolation.driver, "direct");
  assert.equal(result.shell_isolation.isolated, false);
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
  assert.equal(result.shell_isolation.driver, "direct");
  assert.equal(result.shell_isolation.isolated, false);
  assert.equal(result.shell_isolation.fallback, false);
  assert.match(result.output, /done/);
  assert.equal(await readFile(join(root, "result.txt"), "utf8"), "shell-output");
  assert.equal(registry.requiresApproval("local_shell"), false);
});

test("local shell none isolation is explicit direct host execution", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-none-"));
  const registry = createLocalShellToolRegistry({ cwd: root, accessMode: "full", isolation: "none" });

  const result = await registry.execute("local_shell", {
    command: "printf direct",
  });

  assert.equal(result.ok, true);
  assert.equal(result.shell_isolation.executor, "direct");
  assert.equal(result.shell_isolation.driver, "direct");
  assert.equal(result.shell_isolation.isolated, false);
  assert.equal(result.shell_isolation.fallback, false);
  assert.match(result.shell_isolation.warnings.join(" "), /Direct host execution/);
});

test("local shell auto isolation falls back to direct executor with explicit status", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-auto-"));
  const registry = createLocalShellToolRegistry({ cwd: root, accessMode: "full", isolation: "auto" });
  const definition = registry.definitions()[0];
  assert.match(definition.description, /fallback=true/);

  const result = await registry.execute("local_shell", {
    command: "printf isolated-fallback",
  });

  assert.equal(result.ok, true);
  assert.equal(result.shell_isolation.executor, "direct");
  assert.equal(result.shell_isolation.driver, "direct");
  assert.equal(result.shell_isolation.isolated, false);
  assert.equal(result.shell_isolation.fallback, true);
  assert.match(result.shell_isolation.warnings.join(" "), /falling back to direct/);
});

test("local shell isolation options are reported without pretending direct mode enforces them", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-options-"));
  const registry = createLocalShellToolRegistry({
    cwd: root,
    accessMode: "full",
    isolation: "auto",
    isolationOptions: {
      filesystem: "workdir-readwrite",
      network: "blocked",
      env: "minimal",
      resources: { memoryMb: 256, cpuCount: 1 },
    },
  });
  const definition = registry.definitions()[0];
  assert.match(definition.description, /filesystem=workdir-readwrite/);
  assert.match(definition.description, /network=blocked/);
  assert.match(definition.description, /memory_mb=256/);

  const result = await registry.execute("local_shell", {
    command: "printf options",
  });

  assert.equal(result.ok, true);
  assert.deepEqual(result.shell_isolation.requested, {
    filesystem: "workdir-readwrite",
    network: "blocked",
    env: "minimal",
    resources: { memoryMb: 256, cpuCount: 1 },
  });
  assert.equal(result.shell_isolation.guarantees.filesystem, "none");
  assert.equal(result.shell_isolation.guarantees.network, "allowed");
  assert.match(result.shell_isolation.warnings.join(" "), /not enforced by direct execution/);
});

test("local shell required isolation can run through agent-isolator protocol", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-isolator-"));
  const executablePath = await createFakeIsolator(root);
  const registry = createLocalShellToolRegistry({
    cwd: root,
    accessMode: "full",
    isolation: "required",
    isolationOptions: {
      filesystem: "workdir-readwrite",
      network: "blocked",
      env: "minimal",
    },
    isolator: { executablePath, driver: "fake" },
  });
  const definition = registry.definitions()[0];
  assert.match(definition.description, /isolation_driver=fake-isolator/);

  const result = await registry.execute("local_shell", {
    command: "printf through-isolator",
  });

  assert.equal(result.ok, true);
  assert.equal(result.output, "through-isolator");
  assert.equal(result.cwd, root);
  assert.equal(result.shell_isolation.executor, "isolator");
  assert.equal(result.shell_isolation.driver, "fake-isolator");
  assert.equal(result.shell_isolation.isolated, true);
  assert.equal(result.shell_isolation.guarantees.network, "blocked");
});

test("local shell required isolation rejects missing isolating runner", async () => {
  const previousPath = process.env.AGENT_ISOLATOR_PATH;
  delete process.env.AGENT_ISOLATOR_PATH;
  try {
    assert.throws(
      () => createLocalShellToolRegistry({ accessMode: "full", isolation: "required" }),
      /executable path is not configured|AGENT_ISOLATOR_PATH/i,
    );
  } finally {
    if (previousPath == null) {
      delete process.env.AGENT_ISOLATOR_PATH;
    } else {
      process.env.AGENT_ISOLATOR_PATH = previousPath;
    }
  }
  assert.throws(
    () => createLocalShellToolRegistry({
      accessMode: "full",
      isolation: "required",
      runner: {
        async run() {
          throw new Error("should not run");
        },
      },
    }),
    /does not report isolation/,
  );
});

test("local shell required isolation fails closed when agent-isolator is unavailable", async () => {
  assert.throws(
    () => createLocalShellToolRegistry({
      accessMode: "full",
      isolation: "required",
      isolator: { executablePath: join(tmpdir(), "missing-agent-isolator") },
    }),
    /ENOENT|no such file/i,
  );
});

test("local shell auto isolation uses explicit AGENT_ISOLATOR_PATH when configured", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-shell-env-isolator-"));
  const executablePath = await createFakeIsolator(root);
  const previousPath = process.env.AGENT_ISOLATOR_PATH;
  process.env.AGENT_ISOLATOR_PATH = executablePath;
  try {
    const registry = createLocalShellToolRegistry({
      cwd: root,
      accessMode: "full",
      isolation: "auto",
      isolationOptions: {
        filesystem: "workdir-readwrite",
        network: "allowed",
        env: "inherit",
      },
      isolator: { driver: "fake" },
    });
    const result = await registry.execute("local_shell", { command: "printf through-env-isolator" });
    assert.equal(result.shell_isolation.executor, "isolator");
    assert.equal(result.shell_isolation.driver, "fake-isolator");
  } finally {
    if (previousPath == null) {
      delete process.env.AGENT_ISOLATOR_PATH;
    } else {
      process.env.AGENT_ISOLATOR_PATH = previousPath;
    }
  }
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

async function createFakeIsolator(root) {
  const file = join(root, "fake-agent-isolator.mjs");
  await writeFile(file, `#!/usr/bin/env node
const chunks = [];
process.stdin.on("data", (chunk) => chunks.push(chunk));
process.stdin.on("end", () => {
  const request = JSON.parse(Buffer.concat(chunks).toString("utf8"));
  const status = {
    executor: "isolator",
    driver: "fake-isolator",
    isolated: true,
    fallback: false,
    requested: {
      filesystem: "workdir-readwrite",
      network: "blocked",
      env: "minimal",
      resources: {},
    },
    guarantees: {
      filesystem: "workdir-mounted",
      network: "blocked",
      user: "namespace-user",
      process: "pid-namespace",
      resources: "timeout-only",
    },
    warnings: [],
  };
  if (request.method === "status") {
    process.stdout.write(JSON.stringify({
      id: request.id,
      result: {
        version: "test",
        driver: "fake-isolator",
        status,
        drivers: [{ name: "fake-isolator", platform: process.platform, available: true }],
      },
    }));
    return;
  }
  process.stdout.write(JSON.stringify({
    id: request.id,
    result: {
      ok: true,
      action: "run",
      command: request.params.command,
      description: request.params.description,
      cwd: request.params.cwd,
      exit_code: 0,
      signal: null,
      stdout: "through-isolator",
      stderr: "",
      output: "through-isolator",
      duration_ms: 1,
      timed_out: false,
      truncated: false,
      shell_isolation: status,
    },
  }));
});
`);
  await chmod(file, 0o755);
  return file;
}
