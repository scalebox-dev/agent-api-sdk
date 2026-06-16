#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const manifest = JSON.parse(fs.readFileSync(path.join(packageRoot, "scripts", "routes.json"), "utf8"));

function readTree(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...readTree(full));
    } else {
      out.push(full);
    }
  }
  return out;
}

const sources = Object.fromEntries(
  readTree(path.join(packageRoot, "src"))
    .filter((file) => file.endsWith(".ts"))
    .map((file) => [file, fs.readFileSync(file, "utf8")]),
);

function pathNeedles(operationPath) {
  const staticPrefix = operationPath.split("{")[0].replace(/\/$/, "");
  return [operationPath.replace(/\{[^}]+\}/g, ""), staticPrefix].filter(Boolean);
}

function hasPathCoverage(operationPath, methodHint) {
  const needles = pathNeedles(operationPath);
  return Object.values(sources).some((content) => {
    if (!needles.some((needle) => content.includes(needle))) {
      return false;
    }
    if (methodHint === "POST" && operationPath.includes("/cancel")) {
      return content.includes("cancel");
    }
    if (methodHint === "GET" && operationPath.includes("/events")) {
      return content.includes("events");
    }
    if (methodHint === "GET" && operationPath.includes("/children")) {
      return content.includes("children");
    }
    if (methodHint === "GET" && operationPath.includes("/archive")) {
      return content.includes("/archive");
    }
    if (methodHint === "GET" && operationPath.endsWith("/volume")) {
      return content.includes("/volume");
    }
    if (methodHint === "GET" && operationPath.includes("/grep")) {
      return content.includes("/grep");
    }
    if (methodHint === "POST" && operationPath.includes("/summarize")) {
      return content.includes("/summarize");
    }
    if (operationPath.includes("/file_lines/")) {
      return content.includes("/file_lines/");
    }
    if (methodHint === "DELETE" && operationPath.includes("/paths/")) {
      return content.includes("/paths/");
    }
    return true;
  });
}

const failures = [];
for (const operation of manifest.operations) {
  if (!hasPathCoverage(operation.path, operation.method)) {
    failures.push(`Missing coverage for ${operation.method} ${operation.path} (${operation.symbol})`);
  }
}

if (failures.length > 0) {
  console.error("@agent-api/sdk route coverage failed:\n" + failures.map((line) => `- ${line}`).join("\n"));
  process.exit(1);
}

console.log(`@agent-api/sdk route coverage OK (${manifest.operations.length} operations)`);
