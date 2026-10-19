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
async function startServer(
  directory,
  env,
  markers,
  command = process.execPath,
  args = ["dist/server.js"],
) {
  const server = spawn(command, args, {
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

async function stopServer(server) {
  if (server.exitCode !== null || server.signalCode !== null) return;
  const stopped = new Promise((resolve) => server.once("exit", resolve));
  server.kill();
  await stopped;
}

const aiOutput = "An OpenAPI SDK turns your API contract into typed calls.";
function verifyAIClientFailures(command, args, directory, env) {
  for (const prompt of ["upstream-error", "upstream-disconnect"]) {
    const result = spawnSync(command, [...args, prompt], {
      cwd: directory,
      env: { ...process.env, ...env },
      encoding: "utf8",
      timeout: 10_000,
    });
    assert.equal(result.error, undefined);
    assert.equal(result.signal, null);
    assert.notEqual(
      result.status,
      0,
      "The generated client must report stream errors",
    );
    assert.ok(result.stderr.length > 0);
    if (prompt === "upstream-disconnect")
      assert.equal(result.stdout, "An OpenAPI SDK ");
  }
}
async function verifyAIServer(baseURL, providerURL) {
  const post = (body) =>
    fetch(`${baseURL}/generate`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
      signal: AbortSignal.timeout(5_000),
    });
  for (const body of ["{", '{"prompt":""}', '{"prompt":"unused"}{}']) {
    const invalid = await post(body);
    assert.equal(invalid.status, 400);
    await invalid.text();
  }
  const upstreamError = await post(
    JSON.stringify({ prompt: "upstream-error" }),
  );
  assert.equal(upstreamError.status, 502);
  assert.equal((await upstreamError.text()).trim(), "AI generation failed");

  const disconnected = await post(
    JSON.stringify({ prompt: "upstream-disconnect" }),
  );
  assert.equal(disconnected.status, 200);
  await assert.rejects(
    disconnected.text(),
    "Truncated model streams must fail",
  );

  const before = await (await fetch(`${providerURL}/stats`)).json();
  const abort = new AbortController();
  const pending = await fetch(`${baseURL}/generate`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ prompt: "upstream-wait" }),
    signal: abort.signal,
  });
  const reader = pending.body.getReader();
  assert.ok((await reader.read()).value.length > 0);
  abort.abort();
  for (let attempt = 0; attempt < 40; attempt++) {
    const after = await (await fetch(`${providerURL}/stats`)).json();
    if (after.cancellations > before.cancellations) return;
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  assert.fail("Client cancellation did not reach the provider request");
}

let aiProvider;
try {
  const providerURL = "http://127.0.0.1:18976";
  const aiEnv = {
    OPENAI_API_KEY: "example-test-key",
    OPENAI_MODEL: "example-test-model",
    OPENAI_BASE_URL: `${providerURL}/v1`,
  };
  aiProvider = await startServer(
    root,
    { AI_PROVIDER_PORT: "18976" },
    ["AI provider fixture listening"],
    process.execPath,
    [resolve(root, "docs/scripts/ai-provider-fixture.mjs")],
  );
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
      aiOutput,
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
    let server;
    try {
      if (name === "golang-ai-streaming-app") {
        const serverDirectory = resolve(directory, "server");
        const executable = resolve(scratch, "go-ai-server");
        run(
          "go",
          ["build", "-mod=readonly", "-o", executable, "."],
          serverDirectory,
        );
        server = await startServer(
          serverDirectory,
          { ...aiEnv, AI_API_PORT: "18977" },
          ["AI API listening"],
          executable,
          [],
        );
        env.AI_API_BASE_URL = "http://127.0.0.1:18977";
      }
      assert.equal(
        run("go", ["run", "-mod=readonly", "."], directory, env),
        expected,
      );
      if (server) {
        verifyAIClientFailures(
          "go",
          ["run", "-mod=readonly", "."],
          directory,
          env,
        );
        await verifyAIServer(env.AI_API_BASE_URL, providerURL);
      }
    } finally {
      if (server) await stopServer(server);
    }
    console.log(
      `ok downloaded ${name}: standard-library client ran over local HTTP${server ? "; provider SDK stream, errors and cancellation verified" : ""}`,
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
    [
      "typescript-ai-streaming-app",
      "api",
      [],
      {
        ...aiEnv,
        AI_API_PORT: "18978",
        AI_API_BASE_URL: "http://127.0.0.1:18978",
      },
      ["AI API listening"],
    ],
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
    if (name === "typescript-ai-streaming-app")
      run(
        "corepack",
        ["pnpm@12.14.0", "install", "--frozen-lockfile"],
        directory,
      );
    else await symlink(tooling, resolve(directory, "node_modules"), "dir");
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
      else if (name === "typescript-ai-streaming-app") {
        assert.equal(result, aiOutput);
        verifyAIClientFailures(
          process.execPath,
          ["dist/client.js"],
          directory,
          env,
        );
        await verifyAIServer(env.AI_API_BASE_URL, providerURL);
      } else if (name === "typescript-webhook-app")
        assert.deepEqual(JSON.parse(result.split("\n").slice(1).join("\n")), {
          accepted: 204,
          invalidCredentials: 401,
          invalidBody: 400,
        });
      else assert.ok(JSON.parse(result)); // Each consumer verifies its feature results before printing them.
      console.log(`ok downloaded ${name}: generated, compiled and ran`);
    } finally {
      if (server) {
        await stopServer(server);
      }
    }
  }
} finally {
  if (aiProvider) await stopServer(aiProvider);
  await rm(scratch, { recursive: true, force: true });
}
