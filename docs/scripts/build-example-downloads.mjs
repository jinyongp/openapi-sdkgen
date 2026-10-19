import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { zipSync } from "fflate";

const root = resolve(import.meta.dirname, "../..");
const tsFiles = [
  "README.md",
  "openapi.json",
  "package.json",
  "pnpm-lock.yaml",
  "pnpm-workspace.yaml",
  "setup.sh",
  "tsconfig.json",
  "src/client.ts",
  "src/server.ts",
];
const standaloneTSFiles = [
  "README.md",
  "openapi.yaml",
  "package.json",
  "pnpm-lock.yaml",
  "pnpm-workspace.yaml",
  "setup.sh",
  "tsconfig.json",
  "src/client.ts",
];
const goFiles = ["README.md", "openapi.json", "go.mod", "setup.sh", "main.go"];

// Keep archives limited to reviewed source; never include generated output or local files.
export const exampleDownloads = [
  { name: "typescript-todo-app", files: tsFiles },
  { name: "typescript-report-jobs-app", files: [...tsFiles, "orders.json"] },
  {
    name: "typescript-file-conversion-app",
    files: [...tsFiles, "contacts.json"],
  },
  {
    name: "typescript-request-streaming-app",
    files: [...tsFiles, "todos.ndjson", "src/todo.ts"],
  },
  { name: "typescript-advanced-app", files: tsFiles },
  { name: "typescript-openapi-capabilities-app", files: tsFiles },
  {
    name: "typescript-ai-streaming-app",
    files: [...standaloneTSFiles, "src/server.ts"],
  },
  {
    name: "typescript-webhook-app",
    files: [...standaloneTSFiles, "src/worker.ts"],
  },
  { name: "golang-todo-app", files: goFiles },
  {
    name: "golang-file-conversion-app",
    files: [...goFiles, "contacts.json", "cmd/server/main.go"],
  },
  {
    name: "golang-report-jobs-app",
    files: [...goFiles, "orders.json", "cmd/server/main.go"],
  },
  {
    name: "golang-ai-streaming-app",
    files: [...goFiles, "server/go.mod", "server/go.sum", "server/main.go"],
  },
];

export async function buildExampleDownloads(
  destination = resolve(root, "docs/public/downloads"),
) {
  await mkdir(destination, { recursive: true });
  for (const example of exampleDownloads) {
    const entries = {};
    for (const file of example.files)
      entries[`${example.name}/${file}`] = new Uint8Array(
        await readFile(resolve(root, "examples", example.name, file)),
      );
    entries[`${example.name}/LICENSE`] = new Uint8Array(
      await readFile(resolve(root, "LICENSE")),
    );
    const archive = zipSync(entries, { level: 9, mtime: new Date(1980, 0, 1) });
    await writeFile(resolve(destination, `${example.name}.zip`), archive);
  }
  return exampleDownloads.length;
}

if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  console.log(
    `ok downloadable examples: ${await buildExampleDownloads()} source archives`,
  );
}
