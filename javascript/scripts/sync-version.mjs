#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const pkg = JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8"));
const version = pkg.version;
if (!/^\d+\.\d+\.\d+(-[\w.-]+)?$/.test(version)) {
  console.error(`Invalid version in package.json: ${version}`);
  process.exit(1);
}

const versionTs = `export const VERSION = "${version}";\n\nexport const USER_AGENT = \`@agent-api/sdk/\${VERSION}\`;\n`;
fs.writeFileSync(path.join(packageRoot, "src", "version.ts"), versionTs);
console.log(`Synced @agent-api/sdk version ${version} -> src/version.ts`);
