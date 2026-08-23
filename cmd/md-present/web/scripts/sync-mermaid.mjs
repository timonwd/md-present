import { copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const webAssets = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const mermaidPackage = resolve(webAssets, "node_modules/mermaid");

await mkdir(webAssets, { recursive: true });
await Promise.all([
  copyFile(resolve(mermaidPackage, "dist/mermaid.min.js"), resolve(webAssets, "mermaid.min.js")),
  copyFile(resolve(mermaidPackage, "LICENSE"), resolve(webAssets, "mermaid.LICENSE.txt")),
]);
