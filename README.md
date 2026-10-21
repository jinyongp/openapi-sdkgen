# openapi-sdkgen

Generate TypeScript or Go SDK source from OpenAPI 3.0, 3.1 and 3.2 documents.
Use the generated clients, models and validation code inside your application.

[Documentation](https://jinyongp.dev/openapi-sdkgen/) ·
[Runnable examples](https://jinyongp.dev/openapi-sdkgen/typescript/examples/)

## Install

macOS / Linux:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 | iex
```

If prompted, run the printed `PATH` command, then run `openapi-sdkgen --version`.

### Install a specific version

Use the installer attached to the version you want. For example, to install **v1.3.1**:

```sh
curl -fsSL https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.3.1/install.sh | sh
```

```powershell
irm https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.3.1/install.ps1 | iex
```

Replace `v1.3.1` with a tag from [the release list](https://github.com/jinyongp/openapi-sdkgen/releases)
that includes `install.sh` or `install.ps1`.
Each release's installer defaults to that version. You can also share the installer links directly:
[v1.3.1 macOS/Linux](https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.3.1/install.sh) ·
[v1.3.1 Windows](https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.3.1/install.ps1).
With npm, use `npm install -g openapi-sdkgen@1.3.1` (Node.js 22+).

<details>
<summary>Custom installation directory and other installation methods</summary>

The default directory is `~/.local/bin` on macOS/Linux or
`%LOCALAPPDATA%\Programs\openapi-sdkgen\bin` on Windows.
To choose a directory, save the installer first:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh -o install.sh
sh install.sh --install-dir /your/bin
```

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 -OutFile install.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -InstallDir C:\your\bin
```

Use a version-specific installer URL above to choose both the version and directory.
To upgrade to the latest version, rerun the commands at the start of this section.
To uninstall, remove the installed executable.

Other options: [download an executable](https://github.com/jinyongp/openapi-sdkgen/releases),
`npm install -g openapi-sdkgen` (Node.js 22+), or
`brew install jinyongp/tap/openapi-sdkgen`.

</details>

## Quick start: TypeScript

Generate from your `openapi.yaml`:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

Import the generated source and call an operation defined in your contract.
For a Todo API, use the following with your server's `baseURL`:

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "https://api.example.com" });
const todo = await api.todos.create({
  body: { title: "Write documentation" },
});
```

For a server returning the created Todo, an example `todo` value is
`{ id: "1", title: "Write documentation", completed: false }`.

Build the SDK with your application's toolchain. Bundlers such as Vite and
Next.js also accept the directory import `./generated/api`.

[Getting started](https://jinyongp.dev/openapi-sdkgen/typescript/guide/getting-started) ·
[Download the Todo application](https://jinyongp.dev/openapi-sdkgen/downloads/typescript-todo-app.zip)

## Quick start: Go

Generate inside your Go module:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target golang \
  --output ./internal/api \
  --incremental
```

The CLI derives the import path from the output module's `go.mod`.
Import the generated package and build it with Go 1.27+.

[Getting started](https://jinyongp.dev/openapi-sdkgen/golang/guide/getting-started) ·
[Download the Todo application](https://jinyongp.dev/openapi-sdkgen/downloads/golang-todo-app.zip)

## Update the SDK

Run the same command after changing your OpenAPI document. `--incremental`
updates generated files and preserves files you added. Use `--check --output`
in CI to detect stale generated source.

For webhook and callback receivers, add `--with server`. Input can be a local
JSON/YAML file, an HTTP(S) URL, a `file://` URL or `-` for stdin.

## Documentation

- [TypeScript guides](https://jinyongp.dev/openapi-sdkgen/typescript/)
- [Go guides](https://jinyongp.dev/openapi-sdkgen/golang/)
- [TypeScript examples](https://jinyongp.dev/openapi-sdkgen/typescript/examples/)
- [Go examples](https://jinyongp.dev/openapi-sdkgen/golang/examples/)

## License

[Apache License 2.0](LICENSE)
