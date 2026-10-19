# Convert a contact file

Upload `contacts.json` through the generated SDK. A separate HTTP server converts
its JSON array into one contact per NDJSON line; the client saves `contacts.ndjson`.
The server takes each contact's `name` and `email`; other fields are omitted.

Install Go 1.27+, Bash and the `openapi-sdkgen` CLI. From this directory:

```sh
bash ./setup.sh
go run ./cmd/server
```

In another terminal, from this directory:

```sh
go run .
```

```text
Saved: contacts.ndjson
{"name":"Ada Lovelace","email":"ada@example.test"}
{"name":"Grace Hopper","email":"grace@example.test"}
```

Edit `contacts.json` and run the client again to convert different data.
The input must be an array of objects with non-empty string `name` and `email`
fields. Invalid files return HTTP 400; uploads above 64 KiB return HTTP 413.
An empty array converts to an empty file. The client has a ten-second deadline.

The server binds loopback on port `18798`. Override `CONVERSION_API_PORT` and
the client's `CONVERSION_API_BASE_URL` together. Set `SDKGEN_BIN` for a specific
CLI executable. This example converts small files in memory.
