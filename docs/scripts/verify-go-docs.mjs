import { mkdtemp, readFile, writeFile, copyFile, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
const root = resolve(import.meta.dirname, "../..");
const scratch = await mkdtemp(resolve(root, ".tmp/go-docs-"));
function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, encoding: "utf8", env: { ...process.env, GOWORK: "off", GOTOOLCHAIN: "local" } });
  if (result.status !== 0) throw new Error(`${command} ${args.join(" ")}\n${result.error ?? result.stderr}\n${result.stdout}`);
  return result.stdout.trim();
}
try {
  const en = await readFile(resolve(root, "docs/golang/guide/getting-started.md"), "utf8");
  const ko = await readFile(resolve(root, "docs/ko/golang/guide/getting-started.md"), "utf8");
  const code = en.match(/```go\n([\s\S]*?)```/)[1];
  if (code !== ko.match(/```go\n([\s\S]*?)```/)[1]) throw new Error("First-call examples differ by display language");
  await writeFile(resolve(scratch, "main.go"), code);
  const module = await readFile(resolve(root, "examples/golang-todo-app/go.mod"), "utf8");
  await writeFile(resolve(scratch, "go.mod"), module.replace("example.com/sdkgen-go-example", "example.com/consumer"));
  await copyFile(resolve(root, "examples/golang-todo-app/go.sum"), resolve(scratch, "go.sum"));
  run("go", ["build", "-o", resolve(scratch, "generator"), "./cmd/openapi-sdkgen"]);
  run(resolve(scratch, "generator"), ["generate", "--input", resolve(root, "docs/public/examples/golang-todo.json"), "--target", "golang", "--go-import-path", "example.com/consumer/internal/api", "--output", resolve(scratch, "internal/api"), "--with", "server"]);
  const output = run("go", ["run", "-mod=readonly", "."], scratch);
  if (output !== "todo-1 Ship the SDK") throw new Error(`Unexpected first call: ${output}`);
  console.log("ok Go documentation first call: generated, compiled, local HTTP result verified");
} finally { await rm(scratch, { recursive: true, force: true }); }
