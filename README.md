# openapi-sdkgen

Generate TypeScript or Go SDK source from OpenAPI 3.0, 3.1 and 3.2 documents.
Use the generated clients, models and validation code inside your application.

[Documentation](https://jinyongp.dev/openapi-sdkgen/) ·
[Runnable examples](https://jinyongp.dev/openapi-sdkgen/typescript/examples/)

## Install

```sh
brew install jinyongp/tap/openapi-sdkgen
```

Or use `npm install -g openapi-sdkgen` with Node.js 22+.
[Native executables](https://github.com/jinyongp/openapi-sdkgen/releases) are also available.

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
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --incremental
```

Match `--go-import-path` to your module and output directory. Import the generated
package and build it with Go 1.27+.

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
