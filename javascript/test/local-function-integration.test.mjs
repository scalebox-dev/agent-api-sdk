import assert from "node:assert/strict";
import test from "node:test";

import {
  AgentAPI,
  functionCallOutputInput,
  pendingFunctionCalls,
  runLocalFunctionHandlers,
} from "../dist/index.js";

const integrationEnabled = process.env.AGENT_API_INTEGRATION === "1";
const localFunctionEnabled =
  integrationEnabled && process.env.AGENT_API_LOCAL_FUNCTION_TEST === "1";
const apiKey = process.env.AGENT_API_KEY ?? "dev-token";
const baseURL = (process.env.AGENT_API_BASE_URL ?? "http://127.0.0.1:18080").replace(/\/+$/, "");

const SDK_LOCAL_FUNCTION_MARKER = "SDK_LOCAL_FUNCTION_MARKER";

const getWeatherTool = {
  type: "function",
  name: "get_weather",
  description: "Get weather for a city",
  parameters: {
    type: "object",
    properties: { city: { type: "string" } },
    required: ["city"],
  },
};

function integrationClient() {
  return new AgentAPI({
    apiKey,
    baseURL,
    timeout: 180_000,
  });
}

test(
  "integration: local function pause and resume",
  { skip: !localFunctionEnabled },
  async () => {
    const client = integrationClient();

    const paused = await client.responses.create({
      model: "mock-gpt",
      input: `${SDK_LOCAL_FUNCTION_MARKER}: What is the weather in Boston?`,
      tools: [getWeatherTool],
      max_steps: 4,
      max_output_tokens: 256,
    });

    assert.match(paused.id, /^resp_/);
    assert.equal(paused.status, "requires_action");

    const pending = pendingFunctionCalls(paused);
    assert.equal(pending.length, 1);
    assert.equal(pending[0].name, "get_weather");
    assert.equal(pending[0].call_id, "call_get_weather_sdk");

    const outputs = await runLocalFunctionHandlers(paused, {
      get_weather: (args) => `${args.city}: 72F and sunny`,
    });
    assert.equal(outputs.length, 1);
    assert.equal(outputs[0].type, "function_call_output");
    assert.ok(outputs[0].output.includes("72F"));

    const final = await client.responses.create({
      input: outputs,
      previous_response_id: paused.id,
      model: "mock-gpt",
      tools: [getWeatherTool],
      max_steps: 4,
      max_output_tokens: 256,
    });

    assert.equal(final.status, "completed");
    assert.ok((final.output_text ?? "").includes("SDK local function ok"));
    assert.ok((final.output_text ?? "").includes("72F"));

    const retrieved = await client.responses.retrieve(paused.id);
    assert.equal(retrieved.status, "requires_action");
    const functionItems = (retrieved.output ?? []).filter((item) => item.type === "function_call");
    assert.equal(functionItems.length, 1);
    assert.equal(functionItems[0].name, "get_weather");
  },
);

test("unit: SDK local function helpers with mocked HTTP", async () => {
  let callCount = 0;
  const client = new AgentAPI({
    apiKey: "sk-test",
    baseURL: "https://agent.test",
    fetch: async (_url, init) => {
      callCount += 1;
      const body = JSON.parse(String(init?.body ?? "{}"));
      if (callCount === 1) {
        assert.equal(body.tools?.[0]?.type, "function");
        return new Response(
          JSON.stringify({
            id: "resp_paused",
            object: "response",
            created_at: 1,
            status: "requires_action",
            model: "mock-gpt",
            output: [
              {
                type: "function_call",
                id: "fc_1",
                status: "in_progress",
                name: "get_weather",
                call_id: "call_1",
                arguments: JSON.stringify({ city: "Boston" }),
              },
            ],
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        );
      }
      assert.equal(body.previous_response_id, "resp_paused");
      assert.equal(body.input?.[0]?.type, "function_call_output");
      return new Response(
        JSON.stringify({
          id: "resp_final",
          object: "response",
          created_at: 2,
          status: "completed",
          model: "mock-gpt",
          output: [
            {
              type: "message",
              id: "msg_1",
              status: "completed",
              role: "assistant",
              content: [{ type: "output_text", text: "Boston is 72F and sunny." }],
            },
          ],
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      );
    },
  });

  const paused = await client.responses.create({
    model: "mock-gpt",
    input: "weather?",
    tools: [getWeatherTool],
  });
  assert.equal(paused.status, "requires_action");

  const outputs = await runLocalFunctionHandlers(paused, {
    get_weather: (args) => `${args.city}: 72F and sunny`,
  });
  const final = await client.responses.create({
    input: outputs,
    previous_response_id: paused.id,
    model: "mock-gpt",
    tools: [getWeatherTool],
  });

  assert.equal(final.status, "completed");
  assert.equal(final.output_text, "Boston is 72F and sunny.");
  assert.equal(callCount, 2);
});
