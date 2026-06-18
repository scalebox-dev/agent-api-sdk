import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);

test("package exports support ESM imports", async () => {
  const root = await import("@agent-api/sdk");
  const browser = await import("@agent-api/sdk/browser");
  const local = await import("@agent-api/sdk/local");
  const node = await import("@agent-api/sdk/node");

  assert.equal(typeof root.AgentAPI, "function");
  assert.equal(typeof browser.AgentAPI, "function");
  assert.equal(root.localSkillFromDirectory, undefined);
  assert.equal(typeof local.createLocalRuntime, "function");
  assert.equal(typeof node.localSkillFromDirectory, "function");
});

test("package exports support CJS require", () => {
  const root = require("@agent-api/sdk");
  const browser = require("@agent-api/sdk/browser");
  const local = require("@agent-api/sdk/local");
  const node = require("@agent-api/sdk/node");

  assert.equal(typeof root.AgentAPI, "function");
  assert.equal(typeof browser.AgentAPI, "function");
  assert.equal(root.localSkillFromDirectory, undefined);
  assert.equal(typeof local.createLocalRuntime, "function");
  assert.equal(typeof node.localSkillFromDirectory, "function");
});
