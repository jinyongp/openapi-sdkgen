# TypeScript Todo application

Generate an SDK, create a Todo over HTTP and list the stored items. The example
includes a local API server and a separate SDK consumer.

## Run

You need the `openapi-sdkgen` CLI on `PATH`, Node.js 24, pnpm 12 and Bash.
From this directory:

```sh
bash ./setup.sh
```

Then run the server and client in separate terminals:

```sh
# Terminal 1
pnpm run server

# Terminal 2
TODO_API_BASE_URL=http://127.0.0.1:18787/v1 pnpm run client
```

The client prints the created Todo and the list containing it as JSON.
Stop the server when finished.

## Files

- `openapi.json`: the API contract.
- `src/server.ts`: the local API, including creation, storage and filtering.
- `src/client.ts`: calls the generated SDK and checks the returned data.
- `setup.sh`: generates `src/generated/todo-sdk/`, installs dependencies and builds.

Set `SDKGEN_BIN` when running `setup.sh` to choose another generator executable.
After editing the contract, rerun setup. Generated SDK source is compiled together
with the application.
