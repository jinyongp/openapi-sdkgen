# TypeScript SSE consumer

Install the `openapi-sdkgen` CLI, Node.js 24, pnpm 12 and Bash. Run from this directory:

```sh
bash ./setup.sh
pnpm start
```

`openapi.yaml` defines the contract. `setup.sh` generates SDK source and compiles
`src/` with the included TypeScript configuration. Set `SDKGEN_BIN` to use a
specific generator executable.

`src/client.ts` consumes a local SSE response and prints `Hello` and `world`.
No model provider or API key is required. To connect a service, remove the custom
`fetch` and set `baseURL` to its address.

`integration/server.ts` shows an optional AI SDK integration. It is outside the
consumer build: provide your own `ai` installation, configured `model.ts`, and
HTTP host for `POST /generate`. The generated consumer does not depend on them.
