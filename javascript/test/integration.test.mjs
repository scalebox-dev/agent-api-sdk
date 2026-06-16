import assert from "node:assert/strict";
import test from "node:test";

import { AgentAPI } from "../dist/index.js";

const integrationEnabled = process.env.AGENT_API_INTEGRATION === "1";
const apiKey = process.env.AGENT_API_KEY;
const baseURL = (process.env.AGENT_API_BASE_URL ?? "https://api.agentsway.dev").replace(/\/+$/, "");

function integrationClient() {
  if (!apiKey) {
    throw new Error("AGENT_API_KEY is required for integration tests");
  }
  return new AgentAPI({
    apiKey,
    baseURL,
    timeout: 120_000,
  });
}

test("integration: discovery endpoints", { skip: !integrationEnabled }, async () => {
  const client = integrationClient();

  const models = await client.models.list();
  assert.equal(models.object, "list");
  assert.ok(Array.isArray(models.data));
  if (models.data.length > 0) {
    const caps = models.data[0].capabilities ?? {};
    assert.ok("supports_streaming" in caps || Object.keys(caps).length === 0);
  }

  const presets = await client.presets.list();
  assert.equal(presets.object, "list");
  assert.ok(presets.data.some((row) => row.preset === "fast-search"));

  const tools = await client.tools.list();
  assert.equal(tools.object, "list");
  const toolNames = tools.data.map((row) => row.name);
  assert.ok(toolNames.includes("web_search"));
  assert.ok(toolNames.includes("smart_web_search"));
});

test("integration: create, retrieve, list, events", { skip: !integrationEnabled }, async () => {
  const client = integrationClient();

  const created = await client.responses.create({
    preset: "fast-search",
    input: "Reply with exactly: SDK integration ok",
    max_output_tokens: 64,
  });

  assert.match(created.id, /^resp_/);
  assert.equal(created.object, "response");
  assert.ok(["completed", "failed", "in_progress", "cancelled"].includes(created.status));
  assert.ok(typeof created.output_text === "string");

  const retrieved = await client.responses.retrieve(created.id);
  assert.equal(retrieved.id, created.id);

  const listed = await client.responses.list({ limit: 5 });
  assert.equal(listed.object, "list");
  assert.ok(Array.isArray(listed.data));
  assert.ok(listed.data.some((row) => row.id === created.id));

  const children = await client.responses.listChildren(created.id);
  assert.equal(children.object, "list");
  assert.ok(Array.isArray(children.data));

  const events = await client.responses.listEvents(created.id, { view: "timeline" });
  assert.ok(Array.isArray(events.data));
  assert.ok(events.data.length > 0);
  assert.ok(events.data.some((event) => event.type === "response.created" || event.type === "response.completed"));
});

test("integration: streaming emits lifecycle events", { skip: !integrationEnabled }, async () => {
  const client = integrationClient();

  const stream = await client.responses.create({
    preset: "fast-search",
    input: "Say hi in one short sentence.",
    max_output_tokens: 64,
    stream: true,
  });

  const types = new Set();
  for await (const event of stream) {
    types.add(event.type);
  }

  assert.ok(types.has("response.created") || types.has("response.in_progress"));
  assert.ok(types.has("response.completed") || types.has("response.failed"));
});

test("integration: POST /v1/agent", { skip: !integrationEnabled }, async () => {
  const client = integrationClient();

  const created = await client.agent.create({
    preset: "fast-search",
    input: "Reply with exactly: agent endpoint ok",
    max_output_tokens: 64,
  });

  assert.match(created.id, /^resp_/);
  assert.ok(typeof created.output_text === "string");
});
