import assert from "node:assert/strict";
import {
  mkdtemp,
  readFile,
  writeFile,
  mkdir,
  rm,
  symlink,
} from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { spawnSync, spawn } from "node:child_process";
import { unzipSync } from "fflate";
import { exampleDownloads } from "./build-example-downloads.mjs";

const root = resolve(import.meta.dirname, "../..");
const scratch = await mkdtemp(resolve(root, ".tmp/downloaded-examples-"));
function run(command, args, cwd, extraEnv = {}) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    timeout: 120_000,
    env: { ...process.env, ...extraEnv },
  });
  if (result.status !== 0)
    throw new Error(
      `${command} ${args.join(" ")}\n${result.error ?? result.stderr}\n${result.stdout}`,
    );
  return result.stdout.trim();
}
async function unpack(name) {
  const example = exampleDownloads.find((item) => item.name === name);
  const files = unzipSync(
    await readFile(resolve(root, `docs/.vitepress/dist/downloads/${name}.zip`)),
  );
  assert.deepEqual(
    Object.keys(files).sort(),
    [...example.files, "LICENSE"].map((file) => `${name}/${file}`).sort(),
  );
  for (const file of example.files) {
    const path = resolve(scratch, name, file);
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, files[`${name}/${file}`]);
  }
  return resolve(scratch, name);
}
async function startServer(directory, env, markers) {
  const server = spawn(process.execPath, ["dist/server.js"], {
    cwd: directory,
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let output = "";
  server.stdout.on("data", (chunk) => {
    output += chunk;
  });
  server.stderr.on("data", (chunk) => {
    output += chunk;
  });
  try {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(
        () => finish(new Error(`Server readiness timed out: ${output}`)),
        10_000,
      );
      const check = setInterval(() => {
        if (markers.every((marker) => output.includes(marker))) finish();
      }, 25);
      const earlyExit = () => finish(new Error(`Server exited: ${output}`));
      server.once("exit", earlyExit);
      function finish(error) {
        clearTimeout(timer);
        clearInterval(check);
        server.removeListener("exit", earlyExit);
        error ? reject(error) : resolve();
      }
    });
    return server;
  } catch (error) {
    server.kill();
    throw error;
  }
}

try {
  const generator = resolve(scratch, "generator");
  run("go", ["build", "-o", generator, "./cmd/openapi-sdkgen"], root);
  for (const [name, module, addons, expected] of [
    [
      "golang-todo-app",
      "example.com/sdkgen-go-example",
      ["--with", "server"],
      "created: 201\nlinked: todo-1 Ship the SDK\nresource: todo-1\nevent: Ship the SDK\nselected: todo-1\nwebhook: todo-1",
    ],
    [
      "golang-ai-streaming-app",
      "example.com/sdkgen-go-stream-example",
      [],
      "hello",
    ],
  ]) {
    const directory = await unpack(name);
    run(
      generator,
      [
        "generate",
        "--input",
        "openapi.json",
        "--target",
        "golang",
        "--go-import-path",
        `${module}/generated/api`,
        "--output",
        "generated/api",
        ...addons,
      ],
      directory,
    );
    const env = {
      GOWORK: "off",
      GOTOOLCHAIN: "local",
      GOPROXY: "off",
      GOSUMDB: "off",
      CGO_ENABLED: "0",
      GOMODCACHE: resolve(scratch, "module-cache"),
    };
    const modules = JSON.parse(
      run("go", ["list", "-m", "-json", "all"], directory, env),
    );
    assert.equal(modules.Path, module);
    assert.ok(modules.Main && !modules.Replace);
    assert.equal(
      run("go", ["run", "-mod=readonly", "."], directory, env),
      expected,
    );
    console.log(
      `ok downloaded ${name}: generated and ran offline over local HTTP`,
    );
  }
  const tooling = resolve(root, "test/typescript/node_modules");
  const compiler = resolve(tooling, "typescript/bin/tsc");
  for (const [name, output, addons, env, markers] of [
    [
      "typescript-todo-app",
      "todo-sdk",
      [],
      {
        TODO_API_PORT: "18971",
        TODO_API_BASE_URL: "http://127.0.0.1:18971/v1",
      },
      ["Todo API listening"],
    ],
    [
      "typescript-advanced-app",
      "widget-sdk",
      [],
      {
        WIDGET_API_PORT: "18972",
        WIDGET_API_BASE_URL: "http://127.0.0.1:18972/v1",
      },
      ["Widget API listening"],
    ],
    [
      "typescript-openapi-capabilities-app",
      "capabilities-sdk",
      ["--with", "server", "--with", "metadata"],
      {
        CAPABILITIES_API_PORT: "18973",
        CAPABILITIES_WEBHOOK_PORT: "18974",
        CAPABILITIES_API_BASE_URL: "http://127.0.0.1:18973/v1",
        CAPABILITIES_WEBHOOK_BASE_URL: "http://127.0.0.1:18974",
      },
      ["Capability API listening", "Capability inbound host listening"],
    ],
    [
      "typescript-request-streaming-app",
      "api",
      [],
      {
        IMPORT_API_PORT: "18975",
        IMPORT_API_BASE_URL: "http://127.0.0.1:18975",
      },
      ["Import API listening"],
    ],
    ["typescript-ai-streaming-app", "api", [], {}, []],
    ["typescript-webhook-app", "api", ["--with", "server"], {}, []],
  ]) {
    const directory = await unpack(name);
    const input =
      name === "typescript-ai-streaming-app" ||
      name === "typescript-webhook-app"
        ? "openapi.yaml"
        : "openapi.json";
    run(
      generator,
      [
        "generate",
        "--input",
        input,
        "--target",
        "typescript",
        "--output",
        `src/generated/${output}`,
        ...addons,
      ],
      directory,
    );
    await symlink(tooling, resolve(directory, "node_modules"), "dir");
    run(process.execPath, [compiler, "--project", "tsconfig.json"], directory);
    let server;
    try {
      if (markers.length) server = await startServer(directory, env, markers);
      const result = run(process.execPath, ["dist/client.js"], directory, env);
      if (name === "typescript-todo-app") {
        const data = JSON.parse(result);
        assert.equal(data.created.title, "Generate and use this SDK");
        assert.deepEqual(data.listed.items, [data.created]);
      } else if (name === "typescript-request-streaming-app")
        assert.equal(result, "Imported 2 todos");
      else if (name === "typescript-ai-streaming-app")
        assert.equal(result, "Hello\nworld");
      else if (name === "typescript-webhook-app")
        assert.deepEqual(JSON.parse(result.split("\n").slice(1).join("\n")), {
          accepted: 204,
          invalidCredentials: 401,
          invalidBody: 400,
        });
      else assert.ok(JSON.parse(result)); // Each consumer verifies its feature results before printing them.
      console.log(`ok downloaded ${name}: generated, compiled and ran`);
    } finally {
      if (server) {
        const stopped = new Promise((resolve) => server.once("exit", resolve));
        server.kill();
        await stopped;
      }
    }
  }
} finally {
  await rm(scratch, { recursive: true, force: true });
}
