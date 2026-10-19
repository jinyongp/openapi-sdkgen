# TypeScript webhook receiver

Install the `openapi-sdkgen` CLI, Node.js 24, pnpm 12 and Bash. Run from this directory:

```sh
bash ./setup.sh
pnpm start
```

`openapi.yaml` defines the contract. `setup.sh` generates SDK source and compiles
`src/` with the included TypeScript configuration. Set `SDKGEN_BIN` to use a
specific generator executable.

`src/worker.ts` creates the typed receiver and maps the public route.
`src/client.ts` sends local Fetch requests, demonstrating a valid delivery (204),
invalid credentials (401), and an invalid body (400), without starting a listener.
The receiver logs the Todo ID and completion state for the accepted delivery.

For deployment, load the token from your application's secret configuration and
forward incoming requests to `router.fetch(request)` from your HTTP host.
