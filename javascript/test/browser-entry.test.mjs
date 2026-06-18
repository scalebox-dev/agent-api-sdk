import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { dirname, join, normalize } from "node:path";
import test from "node:test";

test("browser entry imports without exposing node-local helpers", async () => {
  const sdk = await import("@agent-api/sdk/browser");

  assert.equal(typeof sdk.AgentAPI, "function");
  assert.equal(sdk.localSkillFromDirectory, undefined);
  assert.equal(sdk.createLocalRuntime, undefined);
});

test("browser entry static graph does not reference node builtins", async () => {
  const visited = new Set();
  const nodeBuiltinRefs = [];
  await visitModule(join(process.cwd(), "dist/index.js"), visited, nodeBuiltinRefs);

  assert.deepEqual(nodeBuiltinRefs, []);
});

async function visitModule(file, visited, nodeBuiltinRefs) {
  const normalized = normalize(file);
  if (visited.has(normalized)) {
    return;
  }
  visited.add(normalized);

  const source = await readFile(normalized, "utf8");
  for (const specifier of moduleSpecifiers(source)) {
    if (specifier.startsWith("node:")) {
      nodeBuiltinRefs.push({ file: normalized, specifier });
      continue;
    }
    if (!specifier.startsWith(".")) {
      continue;
    }
    await visitModule(join(dirname(normalized), specifier), visited, nodeBuiltinRefs);
  }
}

function moduleSpecifiers(source) {
  const specifiers = [];
  const pattern = /\b(?:import|export)\s+(?:[^'"]*?\s+from\s+)?["']([^"']+)["']/g;
  for (;;) {
    const match = pattern.exec(source);
    if (!match) {
      break;
    }
    specifiers.push(match[1]);
  }
  return specifiers;
}
