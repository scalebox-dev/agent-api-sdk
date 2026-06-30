import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  AgentAPI,
  APIConnectionError,
  APIStatusError,
  RateLimitError,
  browserAuthSessionExpiresWithin,
  isSupportedVolumeImageContentType,
  isSupportedVolumeImagePath,
  normalizeVolumeAssetPath,
  resolvePresetTools,
  resolvePresetToolsFromCatalog,
} from "../dist/index.js";
import {
  NodeAgentAPI,
  localSkillFromDirectory,
  pendingLocalSkillCalls,
  runLocalSkillHandlers,
} from "../dist/node.js";

function jsonResponse(body, init = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
    ...init,
  });
}

function mockClient(handler, ClientClass = AgentAPI) {
  return new ClientClass({
    apiKey: "sk-test",
    baseURL: "https://agent.test",
    fetch: handler,
  });
}

test("volume asset helpers normalize private image targets", () => {
  assert.equal(normalizeVolumeAssetPath("/agent-volume/reports/chart.png?cache=1"), "reports/chart.png");
  assert.equal(normalizeVolumeAssetPath("/reports/chart.svg#figure"), "reports/chart.svg");
  assert.equal(normalizeVolumeAssetPath("https://example.test/chart.png"), "");
  assert.equal(normalizeVolumeAssetPath("../secret.png"), "");
  assert.equal(isSupportedVolumeImagePath("/reports/chart.svg"), true);
  assert.equal(isSupportedVolumeImagePath("/reports/table.csv"), false);
  assert.equal(isSupportedVolumeImageContentType("image/svg+xml; charset=utf-8"), true);
  assert.equal(isSupportedVolumeImageContentType("text/html"), false);
});

test("responses.create sends bearer auth and adds output_text", async () => {
  let seen;
  const client = mockClient(async (url, init) => {
    seen = { url, init };
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [
        {
          type: "message",
          id: "msg_test",
          status: "completed",
          role: "assistant",
          content: [{ type: "output_text", text: "hello" }],
        },
      ],
    });
  });

  const response = await client.responses.create({ input: "hello", preset: "fast-search" });

  assert.equal(seen.url, "https://agent.test/v1/responses");
  assert.equal(seen.init.headers.Authorization, "Bearer sk-test");
  assert.equal(JSON.parse(seen.init.body).input, "hello");
  assert.equal(response.output_text, "hello");
});

test("responses.create resolves bearer auth from provider per request", async () => {
  const seen = [];
  let tokenIndex = 0;
  const client = new AgentAPI({
    apiKey: "sk-fallback",
    apiKeyProvider: async () => `sk-dynamic-${++tokenIndex}`,
    baseURL: "https://agent.test",
    fetch: async (_url, init) => {
      seen.push(init.headers.Authorization);
      return jsonResponse({
        id: `resp_${seen.length}`,
        object: "response",
        created_at: 1,
        status: "completed",
        model: "test/model",
        output: [],
      });
    },
  });

  await client.responses.create({ input: "one" });
  await client.responses.create({ input: "two" });

  assert.deepEqual(seen, ["Bearer sk-dynamic-1", "Bearer sk-dynamic-2"]);
});

test("responses.create supports caller abort signal", async () => {
  const controller = new AbortController();
  const client = mockClient(async (_url, init) => {
    return await new Promise((_resolve, reject) => {
      init.signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
    });
  });

  const promise = client.responses.create({ input: "hello" }, { signal: controller.signal, maxRetries: 0 });
  controller.abort();

  await assert.rejects(promise, (error) => {
    assert.ok(error instanceof APIConnectionError);
    assert.match(error.message, /Request aborted/);
    return true;
  });
});

test("responses.create streaming supports caller abort signal", async () => {
  const controller = new AbortController();
  const client = mockClient(async (_url, init) => {
    return await new Promise((_resolve, reject) => {
      init.signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
    });
  });

  const promise = client.responses.create({ input: "hello", stream: true }, { signal: controller.signal, maxRetries: 0 });
  controller.abort();

  await assert.rejects(promise, (error) => {
    assert.ok(error instanceof APIConnectionError);
    assert.match(error.message, /Request aborted/);
    return true;
  });
});

test("responses.create serializes capability preferences and smart_web_search tool", async () => {
  let seen;
  const client = mockClient(async (_url, init) => {
    seen = init;
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({
    input: "hello",
    plan_mode_preference: "preferred",
    sub_agent_preference: "off",
    tools: [{ name: "smart_web_search" }],
  });

  const body = JSON.parse(seen.body);
  assert.equal(body.plan_mode_preference, "preferred");
  assert.equal(body.sub_agent_preference, "off");
  assert.deepEqual(body.tools, [{ name: "smart_web_search" }]);
  assert.equal(body.user, undefined);
});

test("memories resource searches public memory API", async () => {
  let seen;
  const client = mockClient(async (url, init) => {
    seen = { url, init, body: JSON.parse(init.body) };
    return jsonResponse({
      object: "memory_search_result",
      data: [{
        id: "mem_1",
        fact: "User prefers espresso.",
        score: 0.92,
        thread_id: "thread_1",
        response_id: "resp_1",
        metadata: { source: "test" },
      }],
      total: 1,
      rewritten_query: "coffee preference",
    });
  });

  const result = await client.memories.search({
    query: "coffee",
    limit: 5,
    previous_response_id: "resp_1",
    tenant_search: true,
    lang: "en",
    semantic_weight: 0.7,
  });

  assert.equal(seen.url, "https://agent.test/v1/memories/search");
  assert.equal(seen.init.method, "POST");
  assert.deepEqual(seen.body, {
    query: "coffee",
    limit: 5,
    previous_response_id: "resp_1",
    tenant_search: true,
    lang: "en",
    semantic_weight: 0.7,
  });
  assert.equal(result.object, "memory_search_result");
  assert.equal(result.data[0].id, "mem_1");
  assert.equal(result.data[0].metadata.source, "test");
});

test("responses.create rejects duplicate tool names", () => {
  const client = mockClient(async () => jsonResponse({ id: "never" }));

  assert.throws(
    () =>
      client.responses.create({
        input: "hello",
        tools: [{ name: "smart_web_search" }, { name: "smart_web_search", type: "search" }],
      }),
    /duplicate tools\[\]\.name: smart_web_search/,
  );
});

test("agent.create uses POST /v1/agent", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.agent.create({ input: "hello" });
  assert.equal(seenURL, "https://agent.test/v1/agent");
});

test("auth device flow starts, polls, and waits for approval", async () => {
  const calls = [];
  const client = mockClient(async (url, init) => {
    calls.push({ url, body: init.body ? JSON.parse(init.body) : undefined });
    if (url.endsWith("/v1/auth/device/start")) {
      return jsonResponse({
        device_code: "dev_secret",
        user_code: "ABCD1234",
        verification_uri: "https://www.example.test/auth/device",
        verification_uri_complete: "https://www.example.test/auth/device?user_code=ABCD1234",
        expires_at: 4102444800,
        interval_seconds: 1,
      });
    }
    if (url.endsWith("/v1/auth/device/poll") && calls.filter((call) => call.url.endsWith("/poll")).length === 1) {
      return jsonResponse({ status: "pending", message: "authorization pending", interval_seconds: 1, expires_at: 4102444800 });
    }
    return jsonResponse({
      status: "approved",
      access_token: "jwt",
      refresh_token: "refresh",
      access_token_expires_at: 4102441200,
      user_id: "user_1",
      workspace_id: "wrk_1",
      workspace_role: "owner",
      scopes: ["responses:create"],
    });
  });

  const started = await client.auth.startDeviceAuth({ client_name: "Agent CLI" });
  assert.equal(started.device_code, "dev_secret");
  assert.equal(calls[0].url, "https://agent.test/v1/auth/device/start");
  assert.deepEqual(calls[0].body, { client_name: "Agent CLI" });

  const approved = await client.waitForDeviceAuth({
    device_code: started.device_code,
    interval_seconds: 1,
    timeout_ms: 3000,
  });
  assert.equal(approved.status, "approved");
  assert.equal(approved.access_token, "jwt");
  assert.equal(calls.at(-1).body.device_code, "dev_secret");
});

test("auth refresh uses refresh-token cookie and expiry helper detects refresh window", async () => {
  let seen;
  const client = mockClient(async (url, init) => {
    seen = { url, init };
    return jsonResponse({
      access_token: "jwt_next",
      refresh_token: "refresh_next",
      access_token_expires_at: 4102441200,
      user_id: "user_1",
      workspace_id: "wrk_1",
      workspace_role: "owner",
      scopes: ["responses:create"],
    });
  });

  const session = await client.refreshBrowserSession({ refresh_token: "refresh original" });

  assert.equal(seen.url, "https://agent.test/v1/auth/refresh");
  assert.equal(seen.init.headers.Cookie, "agent_api_refresh=refresh%20original");
  assert.equal(session.access_token, "jwt_next");
  assert.equal(browserAuthSessionExpiresWithin({ access_token_expires_at: 100 }, 60_000, 40_001), true);
  assert.equal(browserAuthSessionExpiresWithin({ access_token_expires_at: 100 }, 60_000, 39_999), false);
});

test("responses and agent create serialize volume_id", async () => {
  const bodies = [];
  const client = mockClient(async (_url, init) => {
    bodies.push(JSON.parse(init.body));
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({ input: "hello", volume_id: "vol_test" });
  await client.agent.create({ input: "hello", volume_id: "vol_agent" });

  assert.equal(bodies[0].volume_id, "vol_test");
  assert.equal(bodies[1].volume_id, "vol_agent");
});

test("responses and agent create serialize preferred_sites", async () => {
  const bodies = [];
  const client = mockClient(async (_url, init) => {
    bodies.push(JSON.parse(init.body));
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({
    input: "hello",
    preferred_sites: ["arxiv.org", "nature.com"],
  });
  await client.agent.create({
    input: "hello",
    preferred_sites: ["docs.python.org"],
  });

  assert.deepEqual(bodies[0].preferred_sites, ["arxiv.org", "nature.com"]);
  assert.deepEqual(bodies[1].preferred_sites, ["docs.python.org"]);
});

test("responses and agent create serialize platform and local skills", async () => {
  const bodies = [];
  const client = mockClient(async (_url, init) => {
    bodies.push(JSON.parse(init.body));
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({
    input: "hello",
    skills: [{ skill_id: "skl_docs", branch: "dev" }],
    local_skills: [{ local_skill_id: "local_docs", name: "Docs", root_hint: "/tmp/docs", digest: "sha256:abc" }],
  });
  await client.agent.create({
    input: "hello",
    skills: [{ skill_id: "skl_agent" }],
  });

  assert.deepEqual(bodies[0].skills, [{ skill_id: "skl_docs", branch: "dev" }]);
  assert.deepEqual(bodies[0].local_skills, [
    { local_skill_id: "local_docs", name: "Docs", root_hint: "/tmp/docs", digest: "sha256:abc" },
  ]);
  assert.deepEqual(bodies[1].skills, [{ skill_id: "skl_agent" }]);
});

test("local skill handlers focus local SKILL.md", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "release-triage");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), "# Release Triage\n\nMarker: js-local-skill-marker\n");
  await writeFile(join(skillDir, "examples.md"), "example\n");

  const descriptor = await localSkillFromDirectory(skillDir, {
    id: "local_release_triage",
    name: "Release Triage",
  });
  const response = {
    id: "resp_1",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "m",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "skill",
        call_id: "call_skill_1",
        arguments: JSON.stringify({ action: "focus", skills: [{ skill_ref: descriptor.skill_ref, paths: ["examples.md"] }], max_manifest_chars: 4096, max_file_chars: 100 }),
      },
    ],
  };

  assert.equal(pendingLocalSkillCalls(response).length, 1);
  const outputs = await runLocalSkillHandlers(response, [descriptor]);
  assert.equal(outputs.length, 1);
  assert.equal(outputs[0].type, "function_call_output");
  assert.equal(outputs[0].call_id, "call_skill_1");

  const payload = JSON.parse(outputs[0].output);
  const item = payload.data[0];
  assert.equal(item.ok, true);
  assert.equal(item.skill_ref, descriptor.skill_ref);
  assert.equal(item.skill.skill_ref, descriptor.skill_ref);
  assert.equal(item.skill.local_skill_id, undefined);
  assert.match(item.skill.manifest, /js-local-skill-marker/);
  assert.deepEqual(item.skill.entries.map((entry) => entry.path).sort(), ["SKILL.md", "examples.md"]);
  assert.equal(item.skill.files[0].path, "examples.md");
  assert.equal(item.skill.files[0].content, "example\n");
});

test("local skill handlers accept unified skill focus tool calls", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "local-docs");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), "# Local docs\n\nUse examples.");

  const descriptor = await localSkillFromDirectory(skillDir, { id: "local_docs" });
  const response = {
    id: "resp_1",
    object: "response",
    status: "requires_action",
    model: "m",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "skill",
        call_id: "call_skill_1",
        arguments: JSON.stringify({ action: "focus", skills: [{ skill_ref: descriptor.skill_ref }] }),
      },
    ],
  };

  assert.equal(pendingLocalSkillCalls(response).length, 1);
  const outputs = await runLocalSkillHandlers(response, [descriptor]);
  const payload = JSON.parse(outputs[0].output);
  assert.equal(payload.data[0].ok, true);
  assert.equal(payload.data[0].skill.skill_ref, descriptor.skill_ref);
});

test("local skill handlers return file-level read errors", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "release-triage");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), "# Release Triage\n");

  const descriptor = await localSkillFromDirectory(skillDir, { id: "local_release_triage" });
  const response = {
    id: "resp_1",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "m",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "skill",
        call_id: "call_skill_1",
        arguments: JSON.stringify({ action: "focus", skills: [{ skill_ref: descriptor.skill_ref, paths: ["missing.md", "../outside.md"] }] }),
      },
    ],
  };

  const outputs = await runLocalSkillHandlers(response, [descriptor]);
  const payload = JSON.parse(outputs[0].output);
  assert.deepEqual(payload.data[0].skill.files.map((file) => file.error.code), ["skill_file_not_found", "invalid_skill_file_path"]);
});

test("local skill handlers can skip manifest on follow-up focus", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "release-triage");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), "# Release Triage\n\nMarker: should-not-return\n");
  await writeFile(join(skillDir, "examples.md"), "followup example\n");

  const descriptor = await localSkillFromDirectory(skillDir, { id: "local_release_triage" });
  const response = {
    id: "resp_1",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "m",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "skill",
        call_id: "call_skill_1",
        arguments: JSON.stringify({ action: "focus", skills: [{ skill_ref: descriptor.skill_ref, include_manifest: false, paths: ["examples.md"] }] }),
      },
    ],
  };

  const outputs = await runLocalSkillHandlers(response, [descriptor]);
  const payload = JSON.parse(outputs[0].output);
  assert.equal(payload.data[0].skill.manifest, "");
  assert.equal(payload.data[0].skill.manifest_truncated, false);
  assert.equal(payload.data[0].skill.files[0].content, "followup example\n");
});

test("local skill handlers truncate manifests by characters", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "unicode-skill");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), "ab界🙂cd");

  const descriptor = await localSkillFromDirectory(skillDir, { id: "unicode_skill" });
  const response = {
    id: "resp_1",
    object: "response",
    created_at: 1,
    status: "requires_action",
    model: "m",
    output: [
      {
        type: "function_call",
        id: "fc_1",
        status: "in_progress",
        name: "skill",
        call_id: "call_skill_1",
        arguments: JSON.stringify({ action: "focus", skills: [{ skill_ref: descriptor.skill_ref }], max_manifest_chars: 4 }),
      },
    ],
  };

  const outputs = await runLocalSkillHandlers(response, [descriptor]);
  const payload = JSON.parse(outputs[0].output);
  assert.equal(payload.data[0].skill.manifest, "ab界🙂");
  assert.equal(payload.data[0].skill.manifest_truncated, true);
});

test("local skill handlers reject invalid UTF-8 manifests", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-local-skill-"));
  const skillDir = join(root, "broken-skill");
  await mkdir(skillDir);
  await writeFile(join(skillDir, "SKILL.md"), Buffer.from([0x6f, 0x6b, 0xff]));

  await assert.rejects(() => localSkillFromDirectory(skillDir, { id: "broken_skill" }), /utf-8/i);
});

test("responses.list GETs with query params", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({ object: "list", data: [], has_more: false });
  });

  const out = await client.responses.list({
    limit: 5,
    page_token: "tok",
    safety_identifier: "safe_123",
    user_id: "usr_123",
  });
  assert.equal(
    seenURL,
    "https://agent.test/v1/responses?limit=5&page_token=tok&safety_identifier=safe_123&user_id=usr_123",
  );
  assert.equal(out.object, "list");
});

test("responses.retrieve GETs response by id with safety guard", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      id: "resp_abc",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
      tool_results: [{ tool_name: "web_search", status: "completed" }],
      parent_response_id: "resp_parent",
      root_response_id: "resp_root",
      user_id: "usr_123",
      safety_identifier: "safe_123",
    });
  });

  const out = await client.responses.retrieve("resp_abc", { safety_identifier: "safe_123" });
  assert.equal(seenURL, "https://agent.test/v1/responses/resp_abc?safety_identifier=safe_123");
  assert.equal(out.tool_results?.[0]?.tool_name, "web_search");
  assert.equal(out.parent_response_id, "resp_parent");
  assert.equal(out.user_id, "usr_123");
  assert.equal(out.safety_identifier, "safe_123");
});

test("responses.cancel POSTs cancel URL", async () => {
  let seen;
  const client = mockClient(async (url, init) => {
    seen = { url, init };
    return jsonResponse({ interrupted: true });
  });

  const out = await client.responses.cancel("resp_abc");
  assert.equal(seen.url, "https://agent.test/v1/responses/resp_abc/cancel");
  assert.equal(seen.init.method, "POST");
  assert.ok(!seen.init.body);
  assert.equal(out.interrupted, true);
});

test("responses.listChildren GETs children URL", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({ object: "list", data: [{ id: "resp_child", status: "completed", created_at: 1 }] });
  });

  const out = await client.responses.listChildren("resp_parent");
  assert.equal(seenURL, "https://agent.test/v1/responses/resp_parent/children");
  assert.equal(out.data[0].id, "resp_child");
});

test("responses.listEvents GETs events with query params", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      data: [{ type: "response.created", sequence_number: 0 }],
    });
  });

  const out = await client.responses.listEvents("resp_abc", { after_sequence: 3, view: "full" });
  assert.equal(seenURL, "https://agent.test/v1/responses/resp_abc/events?after_sequence=3&view=full");
  assert.equal(out.data[0].type, "response.created");
});

test("responses.retrieveVolume GETs agent volume URL", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({ volume_id: "vol_workspace", name: "workspace" });
  });

  const out = await client.responses.retrieveVolume("resp_abc");
  assert.equal(seenURL, "https://agent.test/v1/responses/resp_abc/volume");
  assert.equal(out.volume_id, "vol_workspace");
});

test("models.list returns capabilities with supports_* keys", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      object: "list",
      data: [
        {
          id: "test/model",
          object: "model",
          owned_by: "test",
          capabilities: { supports_streaming: true, supports_tools: true },
        },
      ],
    });
  });

  const out = await client.models.list();
  assert.equal(seenURL, "https://agent.test/v1/models");
  assert.equal(out.data[0].capabilities?.supports_streaming, true);
});

test("presets.list GETs /v1/presets", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      object: "list",
      data: [{ preset: "fast-search", policy: { allowed_tools: ["web_search"] } }],
    });
  });

  const out = await client.presets.list();
  assert.equal(seenURL, "https://agent.test/v1/presets");
  assert.equal(out.data[0].preset, "fast-search");
});

test("tools.list GETs /v1/tools", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return jsonResponse({
      object: "list",
      data: [
        { object: "tool", name: "web_search" },
        { object: "tool", name: "smart_web_search" },
      ],
    });
  });

  const out = await client.tools.list();
  assert.equal(seenURL, "https://agent.test/v1/tools");
  assert.equal(out.data.length, 2);
  assert.equal(out.data[1].name, "smart_web_search");
});

test("resolvePresetTools fetches preset defaults and appends caller tools", async () => {
  const calls = [];
  const client = mockClient(async (url) => {
    calls.push(url);
    if (url.endsWith("/v1/presets")) {
      return jsonResponse({
        object: "list",
        data: [{ preset: "pro-search", policy: { allowed_tools: ["smart_web_search", "fetch_url"] } }],
      });
    }
    return jsonResponse({
      object: "list",
      data: [
        {
          object: "tool",
          name: "smart_web_search",
          type: "search",
          description: "Search broadly",
          max_tokens: 4096,
          max_tokens_per_page: 2048,
        },
        { object: "tool", name: "fetch_url", type: "url_reader", description: "Fetch a URL" },
      ],
    });
  });

  const localTool = {
    type: "function",
    name: "local_workdir",
    description: "Operate on the local workdir.",
    parameters: { type: "object" },
  };
  const resolved = await resolvePresetTools(client, {
    preset: "pro-search",
    tools: [localTool],
  });

  assert.deepEqual(calls, ["https://agent.test/v1/presets", "https://agent.test/v1/tools"]);
  assert.equal(resolved.preset.preset, "pro-search");
  assert.deepEqual(
    resolved.tools.map((tool) => tool.name),
    ["smart_web_search", "fetch_url", "local_workdir"],
  );
  assert.equal(resolved.tools[0].type, "search");
  assert.equal(resolved.tools[0].max_tokens, 4096);
  assert.equal(resolved.tools[2].type, "function");
});

test("resolvePresetToolsFromCatalog rejects duplicate tool names by default", () => {
  assert.throws(
    () =>
      resolvePresetToolsFromCatalog({
        preset: "pro-search",
        presets: [{ preset: "pro-search", policy: { allowed_tools: ["smart_web_search", "fetch_url"] } }],
        toolCatalog: [{ object: "tool", name: "smart_web_search", type: "search" }],
        tools: [{ name: "smart_web_search", type: "search", max_tokens: 128 }],
      }),
    /duplicate tools\[\]\.name: smart_web_search/,
  );
});

test("resolvePresetToolsFromCatalog can fail closed on unknown preset tools", () => {
  assert.throws(
    () =>
      resolvePresetToolsFromCatalog({
        preset: "pro-search",
        presets: [{ preset: "pro-search", policy: { allowed_tools: ["missing_tool"] } }],
        toolCatalog: [],
        unknownPresetTool: "error",
      }),
    /preset tool not found in catalog: missing_tool/,
  );
});

test("volumes resource covers volume and file routes", async () => {
  const calls = [];
  const client = mockClient(async (url, init) => {
    calls.push({ url, init });
    if (url.endsWith("/v1/volumes") && init.method === "POST") {
      return jsonResponse({ volume_id: "vol_new", name: "docs" }, { status: 201 });
    }
    if (url.endsWith("/v1/volumes/vol_delete")) {
      return new Response(null, { status: 204 });
    }
    if (url.includes("/paths/old.txt") && init.method === "DELETE") {
      return jsonResponse({ path: "old.txt", recursive: false });
    }
    if (url.includes("/entries")) {
      return jsonResponse({ object: "list", entries: [{ path: "dir/file.txt", is_dir: false, size: 5 }] });
    }
    if (url.includes("/search")) {
      return jsonResponse({ object: "list", entries: [{ path: "dir/match.txt", is_dir: false, size: 7 }] });
    }
    if (url.includes("/files/dir/file%20name.txt") && init.method === "GET") {
      return jsonResponse({
        path: "dir/file name.txt",
        encoding: "text",
        mime_type: "text/plain",
        size: 5,
        truncated: false,
        content: "hello",
      });
    }
    if (url.includes("/files/dir/write.txt") && init.method === "PUT") {
      return jsonResponse({ path: "dir/write.txt", size: 11 });
    }
    if (url.includes("/vol_123")) {
      return jsonResponse({ volume_id: "vol_123", name: "docs" });
    }
    return jsonResponse({ object: "list", data: [{ volume_id: "vol_123", name: "docs" }], next_page_token: "next" });
  });

  const listed = await client.volumes.list({ limit: 1, page_token: "tok", user_id: "usr_123" });
  const created = await client.volumes.create({ name: "docs" });
  const retrieved = await client.volumes.retrieve("vol_123");
  const entries = await client.volumes.listEntries("vol_123", { path: "dir", limit: 2, page_token: "etok" });
  const search = await client.volumes.searchEntries("vol_123", { query: "match", path: "dir" });
  const read = await client.volumes.readFile("vol_123", "dir/file name.txt", { max_bytes: 100 });
  const wrote = await client.volumes.writeFile("vol_123", "dir/write.txt", "hello world");
  const deleted = await client.volumes.deletePath("vol_123", "old.txt");
  await client.volumes.delete("vol_delete");

  assert.equal(calls[0].url, "https://agent.test/v1/volumes?limit=1&page_token=tok&user_id=usr_123");
  assert.equal(calls[1].url, "https://agent.test/v1/volumes");
  assert.equal(JSON.parse(calls[1].init.body).name, "docs");
  assert.equal(calls[3].url, "https://agent.test/v1/volumes/vol_123/entries?path=dir&limit=2&page_token=etok");
  assert.equal(calls[4].url, "https://agent.test/v1/volumes/vol_123/search?query=match&path=dir");
  assert.equal(calls[5].url, "https://agent.test/v1/volumes/vol_123/files/dir/file%20name.txt?max_bytes=100");
  assert.equal(calls[6].init.body, "hello world");
  assert.equal(calls[6].init.headers["Content-Type"], undefined);
  assert.equal(calls[7].init.method, "DELETE");
  assert.equal(calls[8].init.method, "DELETE");
  assert.equal(listed.next_page_token, "next");
  assert.equal(created.volume_id, "vol_new");
  assert.equal(retrieved.volume_id, "vol_123");
  assert.equal(entries.entries[0].path, "dir/file.txt");
  assert.equal(search.entries[0].path, "dir/match.txt");
  assert.equal(read.encoding, "text");
  assert.equal(read.content, "hello");
  assert.equal(wrote.size, 11);
  assert.equal(deleted.recursive, false);
});

test("volumes update reconcile and directory routes", async () => {
  const calls = [];
  const client = mockClient(async (url, init) => {
    calls.push({ url, init });
    if (url.includes("/usage/reconcile")) {
      return jsonResponse({ volume_id: "vol_123", name: "renamed", bytes_used: 99 });
    }
    if (url.includes("/directories") && init.method === "POST") {
      return jsonResponse({ path: "notes/archive" }, { status: 201 });
    }
    if (init.method === "PATCH") {
      return jsonResponse({ volume_id: "vol_123", name: "renamed" });
    }
    return jsonResponse({ volume_id: "vol_123", name: "docs" });
  });

  const updated = await client.volumes.update("vol_123", { name: "renamed" });
  const reconciled = await client.volumes.reconcileUsage("vol_123");
  const dir = await client.volumes.createDirectory("vol_123", "notes/archive");

  assert.equal(calls[0].init.method, "PATCH");
  assert.equal(JSON.parse(calls[0].init.body).name, "renamed");
  assert.equal(calls[1].url, "https://agent.test/v1/volumes/vol_123/usage/reconcile");
  assert.equal(calls[2].init.method, "POST");
  assert.equal(JSON.parse(calls[2].init.body).path, "notes/archive");
  assert.equal(updated.name, "renamed");
  assert.equal(reconciled.bytes_used, 99);
  assert.equal(dir.path, "notes/archive");
});

test("volumes readFile supports format=raw", async () => {
  const client = mockClient(async (url) => {
    assert.ok(url.includes("format=raw"));
    return new Response(new Uint8Array([1, 2, 3]), {
      status: 200,
      headers: {
        "Content-Type": "application/octet-stream",
        "X-Volume-Size": "3",
        "X-Volume-Truncated": "false",
      },
    });
  });

  const raw = await client.volumes.readFile("vol_123", "bin/data.bin", { format: "raw" });
  assert.equal(raw.size, 3);
  assert.equal(raw.truncated, false);
  assert.equal(raw.content.byteLength, 3);
  assert.equal(raw.content_type, "application/octet-stream");
});

test("volumes workbench routes cover summarize grep and line edits", async () => {
  const calls = [];
  const client = mockClient(async (url, init) => {
    calls.push({ url, init });
    if (url.endsWith("/summarize")) {
      return jsonResponse({
        summary_path: ".agent-volume/summary.json",
        file_count: 2,
        total_bytes: 20,
        top_paths_by_size: ["a.txt"],
        text_previews: [{ path: "a.txt", size: 10, preview: "hi" }],
        generated_at_unix: 1710000000,
      });
    }
    if (url.includes("/grep?")) {
      return jsonResponse({
        object: "list",
        matches: [{ path: "a.txt", line_number: 1, line: "match" }],
        files_scanned: 1,
        scan_truncated: false,
      });
    }
    if (url.includes("/file_lines/readme.md") && init.method === "GET") {
      return jsonResponse({
        path: "readme.md",
        start_line: 1,
        end_line: 2,
        total_lines: 10,
        lines: ["# Title", ""],
        file_truncated: false,
        size: 12,
      });
    }
    if (url.includes("/file_lines/readme.md") && init.method === "PATCH") {
      return jsonResponse({ path: "readme.md", start_line: 1, end_line: 1, total_lines: 10, size: 13 });
    }
    return jsonResponse({ volume_id: "vol_123" });
  });

  const summary = await client.volumes.summarize("vol_123", { path: "docs" });
  const grep = await client.volumes.grep("vol_123", { pattern: "match", path: "docs", limit: 5 });
  const lines = await client.volumes.readLines("vol_123", "readme.md", { start_line: 1, end_line: 2 });
  const patched = await client.volumes.patchLines("vol_123", "readme.md", {
    start_line: 1,
    end_line: 1,
    replacement: "# Updated",
  });

  assert.equal(calls[0].url, "https://agent.test/v1/volumes/vol_123/summarize");
  assert.equal(JSON.parse(calls[0].init.body).path, "docs");
  assert.ok(calls[1].url.includes("/grep?pattern=match"));
  assert.ok(calls[2].url.includes("start_line=1"));
  assert.equal(JSON.parse(calls[3].init.body).replacement, "# Updated");
  assert.equal(summary.file_count, 2);
  assert.equal(grep.matches[0].line, "match");
  assert.equal(lines.lines[0], "# Title");
  assert.equal(patched.size, 13);
});

test("volumes.downloadArchive GETs archive URL and returns zip bytes", async () => {
  let seenURL;
  const client = mockClient(async (url) => {
    seenURL = url;
    return new Response(new Uint8Array([0x50, 0x4b, 0x03, 0x04]), {
      status: 200,
      headers: { "Content-Type": "application/zip" },
    });
  });

  const archive = await client.volumes.downloadArchive("vol_123", { path: "/notes/drafts/" });
  assert.equal(seenURL, "https://agent.test/v1/volumes/vol_123/archive?path=notes%2Fdrafts");
  assert.equal(archive.path, "notes/drafts");
  assert.equal(archive.content.byteLength, 4);
  assert.equal(archive.content_type, "application/zip");
});

test("skills resource covers management and file routes", async () => {
  const calls = [];
  const client = mockClient(async (url, init) => {
    calls.push({ url, init });
    if (url.endsWith("/v1/skills") && init.method === "POST") {
      return jsonResponse({ skill_id: "skl_new", name: "Docs" }, { status: 201 });
    }
    if (url.endsWith("/v1/skills/discover")) {
      return jsonResponse({ object: "list", data: [{ skill_id: "skl_1", branch: "dev", name: "Docs" }] });
    }
    if (url.endsWith("/v1/skills/focus")) {
      return jsonResponse({ object: "skill_focus_result", data: [{ ok: true, skill_id: "skl_1", branch: "dev", skill: { object: "focused_skill", skill_id: "skl_1", branch: "dev", manifest: "# Skill" } }] });
    }
    if (url.endsWith("/v1/skills/create_dev")) {
      return jsonResponse({ object: "skill_create_result", skill: { skill_id: "skl_created" }, branch: "dev", files: [{ path: "SKILL.md", size: 7 }] }, { status: 201 });
    }
    if (url.endsWith("/v1/skills/update_file")) {
      return jsonResponse({ object: "skill_update_result", data: [{ ok: true, skill_id: "skl_1", path: "SKILL.md", size: 9, skill: { skill_id: "skl_1", branch: "dev" } }] });
    }
    if (url.includes("/files/guide%20book/SKILL.md") && init.method === "GET") {
      return jsonResponse({ path: "guide book/SKILL.md", branch: "dev", content: "# Skill", size: 7 });
    }
    if (url.includes("/files/guide%20book/SKILL.md") && init.method === "PUT") {
      return jsonResponse({ path: "guide book/SKILL.md", branch: "dev", size: 9 });
    }
    if (url.includes("/files?")) {
      return jsonResponse({ object: "list", entries: [{ path: "SKILL.md", is_dir: false, size: 7 }] });
    }
    if (url.includes("/accept_dev")) {
      return jsonResponse({ skill_id: "skl_1", has_dev: false, main_digest: "sha256:dev" });
    }
    if (url.includes("/discard_dev")) {
      return jsonResponse({ skill_id: "skl_1", has_dev: false });
    }
    if (url.includes("/skl_1") && init.method === "PATCH") {
      return jsonResponse({ skill_id: "skl_1", name: "Renamed" });
    }
    if (url.includes("/skl_1/archive")) {
      return jsonResponse({ skill_id: "skl_1", archived: true });
    }
    if (url.includes("/skl_1/export")) {
      return new Response(new Uint8Array([0x50, 0x4b, 0x03, 0x04]), {
        status: 200,
        headers: { "Content-Type": "application/zip" },
      });
    }
    if (url.includes("/skl_1/import")) {
      return jsonResponse({ object: "skill_import_result", branch: "dev", file_count: 2, byte_count: 12, skill: { skill_id: "skl_1" } });
    }
    if (url.includes("/skl_1/diff")) {
      return jsonResponse({ object: "skill_branch_diff", skill_id: "skl_1", base_branch: "main", compare_branch: "dev", summary: { added: 1, modified: 0, deleted: 0, unchanged: 0 }, files: [{ path: "SKILL.md", status: "added" }] });
    }
    if (url.includes("/skl_1") && init.method === "DELETE") {
      return jsonResponse({ deleted: true });
    }
    if (url.includes("/skl_1")) {
      return jsonResponse({ skill_id: "skl_1", name: "Docs" });
    }
    return jsonResponse({ object: "list", data: [{ skill_id: "skl_1" }], next_page_token: "next" });
  });

  const listed = await client.skills.list({ limit: 1, page_token: "tok", user_id: "usr_123" });
  const created = await client.skills.create({ name: "Docs" });
  const discovered = await client.skills.discover({ query: "docs", branch: "dev" });
  const focused = await client.skills.focus({ skills: [{ skill_id: "skl_1", branch: "dev" }] });
  const createdDev = await client.skills.createDev({ name: "New Skill", files: [{ path: "SKILL.md", content: "# Skill" }] });
  const updatedFile = await client.skills.updateFile({ updates: [{ skill_id: "skl_1", path: "SKILL.md", content: "# Skill" }] });
  const retrieved = await client.skills.retrieve("skl_1");
  const updated = await client.skills.update("skl_1", { name: "Renamed" });
  const accepted = await client.skills.acceptDev("skl_1", { strategy: "mirror" });
  const discarded = await client.skills.discardDev("skl_1");
  const files = await client.skills.listFiles("skl_1", { path: "guide book", branch: "dev" });
  const read = await client.skills.readFile("skl_1", "guide book/SKILL.md", { branch: "dev", max_bytes: 100 });
  const wrote = await client.skills.writeFile("skl_1", "guide book/SKILL.md", "# Skill", { branch: "dev" });
  await client.skills.deleteFile("skl_1", "guide book/SKILL.md", { branch: "dev" });
  const archived = await client.skills.archive("skl_1");
  const exported = await client.skills.exportArchive("skl_1", { path: "/examples/", branch: "main" });
  const imported = await client.skills.importArchive("skl_1", new Uint8Array([0x50, 0x4b]).buffer, { path: "examples", branch: "dev", replace: true });
  const diff = await client.skills.diff("skl_1", { path: "/", max_file_chars: 100 });
  const deleted = await client.skills.delete("skl_1");

  assert.equal(calls[0].url, "https://agent.test/v1/skills?limit=1&page_token=tok&user_id=usr_123");
  assert.equal(JSON.parse(calls[1].init.body).name, "Docs");
  assert.equal(calls[11].url, "https://agent.test/v1/skills/skl_1/files/guide%20book/SKILL.md?branch=dev&max_bytes=100");
  assert.equal(calls[12].init.body, "# Skill");
  assert.equal(listed.next_page_token, "next");
  assert.equal(created.skill_id, "skl_new");
  assert.equal(discovered.data[0].branch, "dev");
  assert.equal(focused.data[0].skill.manifest, "# Skill");
  assert.equal(createdDev.skill.skill_id, "skl_created");
  assert.equal(updatedFile.data[0].size, 9);
  assert.equal(retrieved.skill_id, "skl_1");
  assert.equal(updated.name, "Renamed");
  assert.equal(calls[8].url, "https://agent.test/v1/skills/skl_1/accept_dev?strategy=mirror");
  assert.equal(accepted.main_digest, "sha256:dev");
  assert.equal(discarded.has_dev, false);
  assert.equal(files.entries[0].path, "SKILL.md");
  assert.equal(read.content, "# Skill");
  assert.equal(wrote.size, 9);
  assert.equal(archived.archived, true);
  assert.equal(exported.content.byteLength, 4);
  assert.equal(imported.file_count, 2);
  assert.equal(diff.summary.added, 1);
  assert.equal(deleted.deleted, true);
  assert.equal(calls[14].url, "https://agent.test/v1/skills/skl_1/archive");
  assert.equal(calls[15].url, "https://agent.test/v1/skills/skl_1/export?path=examples&branch=main");
  assert.equal(calls[16].url, "https://agent.test/v1/skills/skl_1/import?path=examples&branch=dev&replace=true");
  assert.equal(calls[17].url, "https://agent.test/v1/skills/skl_1/diff?max_file_chars=100");
  assert.equal(calls[18].url, "https://agent.test/v1/skills/skl_1");
  assert.equal(calls[18].init.method, "DELETE");
});

test("skills directory sync helpers push and pull local folders", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-sdk-skill-sync-"));
  const source = join(root, "source");
  const target = join(root, "target");
  await mkdir(join(source, "examples"), { recursive: true });
  await writeFile(join(source, "SKILL.md"), "# Local Skill\n");
  await writeFile(join(source, "examples", "demo.md"), "demo\n");

  let archiveBody;
  const client = mockClient(async (url, init) => {
    if (url.includes("/import")) {
      archiveBody = await new Response(init.body).arrayBuffer();
      return jsonResponse({ object: "skill_import_result", branch: "dev", file_count: 2, byte_count: archiveBody.byteLength, skill: { skill_id: "skl_1" } });
    }
    if (url.includes("/export")) {
      return new Response(archiveBody, { status: 200, headers: { "Content-Type": "application/zip" } });
    }
    throw new Error(`unexpected URL ${url}`);
  }, NodeAgentAPI);

  const pushed = await client.skills.pushDirectory("skl_1", source, { branch: "dev", replace: true });
  const pulled = await client.skills.pullDirectory("skl_1", target, { branch: "dev", replace: true });

  assert.equal(pushed.file_count, 2);
  assert.equal(pulled.file_count, 2);
  assert.equal(await readFile(join(target, "SKILL.md"), "utf8"), "# Local Skill\n");
  assert.equal(await readFile(join(target, "examples", "demo.md"), "utf8"), "demo\n");
});

test("localSkillFromDirectory describes a local skill tree", async () => {
  const root = await mkdtemp(join(tmpdir(), "agent-skill-"));
  await mkdir(join(root, "nested"));
  await writeFile(join(root, "SKILL.md"), "# Skill\n");
  await writeFile(join(root, "nested", "notes.txt"), "hello\n");

  const descriptor = await localSkillFromDirectory(root, { id: "local_test", name: "Local Test" });

  assert.equal(descriptor.local_skill_id, "local_test");
  assert.match(descriptor.skill_ref, /^local::local_test@[a-f0-9]{16}::main$/);
  assert.equal(descriptor.name, "Local Test");
  assert.equal(descriptor.root_hint, root);
  assert.equal(descriptor.manifest, "# Skill\n");
  assert.equal(descriptor.manifest_truncated, false);
  assert.match(descriptor.digest, /^sha256:[a-f0-9]{64}$/);
});

test("streaming responses parse public SSE events", async () => {
  const client = new AgentAPI({
    apiKey: "sk-test",
    baseURL: "https://agent.test",
    fetch: async () =>
      new Response(
        'event: response.output_text.delta\ndata: {"type":"response.output_text.delta","sequence_number":1,"delta":"hi"}\n\n' +
          'data: {"type":"response.requires_action","sequence_number":2,"response":{"status":"requires_action"}}\n\n' +
          'data: {"type":"response.tool.invocation.completed","sequence_number":3,"tool_result":{"tool_name":"web_search"}}\n\n',
        {
          status: 200,
          headers: { "content-type": "text/event-stream" },
        },
      ),
  });

  const stream = await client.responses.create({ input: "hello", stream: true });
  const events = [];
  for await (const event of stream) {
    events.push(event);
  }

  assert.equal(events.length, 3);
  assert.equal(events[0].type, "response.output_text.delta");
  assert.equal(events[0].delta, "hi");
  assert.equal(events[1].type, "response.requires_action");
  assert.equal(events[1].response?.status, "requires_action");
  assert.equal(events[2].type, "response.tool.invocation.completed");
  assert.equal(events[2].tool_result?.tool_name, "web_search");
});

test("status errors expose status and parsed body", async () => {
  const client = mockClient(async () =>
    jsonResponse({ error: { message: "rate limited", code: "rate_limit_exceeded" } }, { status: 429 }),
  );

  await assert.rejects(
    () => client.responses.create({ input: "hello" }),
    (error) => error instanceof APIStatusError && error.status === 429 && error.message === "rate limited",
  );
});

test("requests include User-Agent header", async () => {
  let headers;
  const client = mockClient(async (_url, init) => {
    headers = init.headers;
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({ input: "hello" });
  assert.match(headers["User-Agent"], /^@agent-api\/sdk\//);
});

test("listPage auto-paginates with listIterator", async () => {
  let call = 0;
  const client = mockClient(async (url) => {
    call += 1;
    if (call === 1) {
      assert.equal(url, "https://agent.test/v1/responses?limit=1");
      return jsonResponse({ object: "list", data: [{ id: "resp_1", status: "completed", created_at: 1 }], has_more: true, next_page_token: "tok" });
    }
    assert.equal(url, "https://agent.test/v1/responses?limit=1&page_token=tok");
    return jsonResponse({ object: "list", data: [{ id: "resp_2", status: "completed", created_at: 2 }], has_more: false });
  });

  const ids = [];
  for await (const item of client.responses.listIterator({ limit: 1 })) {
    ids.push(item.id);
  }
  assert.deepEqual(ids, ["resp_1", "resp_2"]);
});

test("retries retryable 503 responses", async () => {
  let attempts = 0;
  const client = mockClient(async () => {
    attempts += 1;
    if (attempts === 1) {
      return jsonResponse({ error: { message: "upstream unavailable" } }, { status: 503 });
    }
    return jsonResponse({
      id: "resp_test",
      object: "response",
      created_at: 1,
      status: "completed",
      model: "test/model",
      output: [],
    });
  });

  await client.responses.create({ input: "hello" });
  assert.equal(attempts, 2);
});
