# Stream a Todo import to an HTTP server

Install the `openapi-sdkgen` CLI, Node.js 24, pnpm 12 and Bash. Run from this directory:

```sh
bash ./setup.sh
pnpm server
```

In a second terminal, run `pnpm client` from this directory.

The client reads `todos.ndjson` line by line and sends one NDJSON request to
`POST /todos/import`. The server reads each line and prints:

```text
Received: Write documentation
Received: Ship the SDK
```

After the request body ends, the server replies with HTTP `200` and JSON
`{"imported":2}`. The client prints:

```text
Imported 2 todos
```

The response reports how many records the server read; it does not echo the
uploaded records. This example logs records rather than persisting them.
Replace that processing with your application's import logic. It is not an
atomic transaction: an interrupted upload may already have processed records.

The generated client validates each item against `itemSchema` before encoding
it as NDJSON. File reading is application code in `src/client.ts`; receiving
and counting records is application code in `src/server.ts`. The SDK is generated
from `openapi.json`. The generator does not run a server or create application data.
Set `SDKGEN_BIN` to choose a generator executable, or `IMPORT_API_PORT` and
`IMPORT_API_BASE_URL` to use another local port. The default port is 18792.
