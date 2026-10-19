import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
const root = resolve(import.meta.dirname, "../..");
const scratch = await mkdtemp(resolve(root, ".tmp/go-docs-"));
function run(command, args, cwd = root, consumer = false) {
  const result = spawnSync(command, args, { cwd, encoding: "utf8", env: { ...process.env, GOWORK: "off", GOTOOLCHAIN: "local", ...(consumer ? { GOPROXY: "off", GOSUMDB: "off", CGO_ENABLED: "0", GOMODCACHE: resolve(scratch, "module-cache") } : {}) } });
  if (result.status !== 0) throw new Error(`${command} ${args.join(" ")}\n${result.error ?? result.stderr}\n${result.stdout}`);
  return result.stdout.trim();
}
try {
  run("go", ["mod", "init", "example.com/consumer"], scratch);
  run("go", ["build", "-o", resolve(scratch, "generator"), "./cmd/openapi-sdkgen"]);
  const examples = [
    { path: "guide/getting-started", input: "golang-todo", expected: "todo-1 Ship the SDK", addons: ["--with", "server"] },
    { path: "examples/ai-streaming", input: "golang-ai-stream", expected: "hello", addons: [] },
    { path: "examples/webhook-server", input: "golang-todo", expected: "created: 201\nlinked: todo-1 Ship the SDK\nresource: todo-1\nevent: Ship the SDK\nselected: todo-1\nwebhook: todo-1", addons: ["--with", "server"] },
  ];
  for (const example of examples) {
    const en = await readFile(resolve(root, `docs/golang/${example.path}.md`), "utf8");
    const ko = await readFile(resolve(root, `docs/ko/golang/${example.path}.md`), "utf8");
    const code = en.match(/```go\n([\s\S]*?)```/)[1];
    if (code !== ko.match(/```go\n([\s\S]*?)```/)[1]) throw new Error(`Display-language mismatch: ${example.path}`);
    await writeFile(resolve(scratch, "main.go"), code);
    await rm(resolve(scratch, "internal"), { recursive: true, force: true });
    run(resolve(scratch, "generator"), ["generate", "--input", resolve(root, `docs/public/examples/${example.input}.json`), "--target", "golang", "--go-import-path", "example.com/consumer/internal/api", "--output", resolve(scratch, "internal/api"), ...example.addons]);
    const modules = JSON.parse(run("go", ["list", "-m", "-json", "all"], scratch, true));
    if (!modules.Main || modules.Path !== "example.com/consumer" || modules.Replace) throw new Error("Go docs consumer acquired external modules");
    const output = run("go", ["run", "-mod=readonly", "."], scratch, true);
    if (output !== example.expected) throw new Error(`Unexpected ${example.path}: ${output}`);
    console.log(`ok Go documentation ${example.path}: generated, compiled, local HTTP result verified`);
  }
} finally { await rm(scratch, { recursive: true, force: true }); }
