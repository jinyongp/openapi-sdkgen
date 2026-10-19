# Getting started

<span id="_1-prepare-the-environment-and-project"></span>
<span id="_2-save-the-todo-document"></span>
<span id="_3-generate-the-sdk"></span>
<span id="_4-optional-verify-calls-with-mock-responses"></span>
<span id="_5-connect-a-real-api-and-continue"></span>
<span id="download-an-executable"></span>
<span id="homebrew"></span>
<span id="npm"></span>

Generate an SDK from your OpenAPI document and import it into your project.
For a runnable application, [download the Todo example](/downloads/typescript-todo-app.zip)
and follow its README. It includes a local API server and client.

## Install the CLI {#install}

macOS / Linux:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 | iex
```

If prompted, run the printed `PATH` command, then run
`openapi-sdkgen --version`.

::: details Version, location and other installation methods

The default directory is `~/.local/bin` on macOS/Linux or
`%LOCALAPPDATA%\Programs\openapi-sdkgen\bin` on Windows.

To choose a version or directory, save the installer first:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh -o install.sh
sh install.sh --version X.Y.Z --install-dir /your/bin
```

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 -OutFile install.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version X.Y.Z -InstallDir C:\your\bin
```

Replace `X.Y.Z` with a released version.
Download the installer again to upgrade. To uninstall, remove the installed executable.

Other options: [download an executable](https://github.com/jinyongp/openapi-sdkgen/releases),
`npm install -g openapi-sdkgen` (Node.js 22+), or `brew install jinyongp/tap/openapi-sdkgen`.

:::

## Generate the SDK {#generate}

Use your project's `openapi.json`, or try the
[Todo contract](https://github.com/jinyongp/openapi-sdkgen/blob/main/examples/typescript-todo-app/openapi.json).

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

Build the generated `src/generated/api/` source with your application's toolchain.
Run the same command to update it when the contract changes.

## Call the API {#call}

With the Todo contract, create and list items as follows. Set `baseURL` to your API
server; the address below belongs to the downloaded example's local server.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "http://localhost:18787/v1" });

const todo = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list({ query: { completed: false } });
console.log(JSON.stringify(todo));
console.log(JSON.stringify(todos));
```

With a fresh example server, the first call creates item `1` and the second returns it in `items`:

```text
{"id":"1","title":"Write documentation","completed":false}
{"items":[{"id":"1","title":"Write documentation","completed":false}]}
```

`todo` is the created item; `todos.items` contains the listed items.
Bundlers such as Vite and Next.js can import `./generated/api`. The explicit
`/index.js` path above is for compiled Node ESM.

Continue with [calls and errors](./client.md), [authentication and transport](./transport.md),
and [regeneration](./generate.md).
