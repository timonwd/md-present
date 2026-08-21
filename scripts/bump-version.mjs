import { readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const versionPattern = /^(\d+)\.(\d+)\.(\d+)$/;
const requestedVersion = args[0];
if (args.length > 1 || (requestedVersion && !versionPattern.test(requestedVersion))) {
  throw new Error("usage: node scripts/bump-version.mjs [version]");
}
const files = [
  { path: "cmd/md-present/main.go", pattern: /var version = "([^"]+)"/g },
  { path: "plugins/md-present/.codex-plugin/plugin.json", pattern: /"version":\s*"([^"]+)"/g },
  { path: "plugins/md-present/.claude-plugin/plugin.json", pattern: /"version":\s*"([^"]+)"/g },
  { path: ".claude-plugin/marketplace.json", pattern: /"version":\s*"([^"]+)"/g },
];

const sources = await Promise.all(files.map(async (file) => ({
  ...file,
  absolutePath: resolve(root, file.path),
  source: await readFile(resolve(root, file.path), "utf8"),
})));

for (const file of sources) {
  const matches = [...file.source.matchAll(file.pattern)];
  if (matches.length !== 1) {
    throw new Error(`${file.path}: expected exactly one version field, found ${matches.length}`);
  }
  file.version = matches[0][1];
}

const versions = new Set(sources.map((file) => file.version));
if (versions.size !== 1) {
  throw new Error(`version fields are not aligned: ${sources.map((file) => `${file.path}=${file.version}`).join(", ")}`);
}

const currentVersion = sources[0].version;
const match = versionPattern.exec(currentVersion);
if (!match) {
  throw new Error(`current version ${currentVersion} is not a plain semantic version`);
}
const targetVersion = requestedVersion || `${match[1]}.${match[2]}.${BigInt(match[3]) + 1n}`;

if (currentVersion !== targetVersion) {
  await Promise.all(sources.map((file) => {
    file.pattern.lastIndex = 0;
    const updated = file.source.replace(file.pattern, (field, version) => field.replace(version, targetVersion));
    return writeFile(file.absolutePath, updated);
  }));
}

console.log(targetVersion);
