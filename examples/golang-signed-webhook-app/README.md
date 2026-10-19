# Receive signed order webhooks

A separate sender signs an order event and sends it over HTTP. The receiver verifies
the original bytes before the generated webhook handler validates JSON and records
the order. The sender uses a generated SDK client to read the resulting order list.

Install Go 1.27+, Bash and the `openapi-sdkgen` CLI. From this directory:

```sh
bash ./setup.sh
export WEBHOOK_SECRET=local-example-secret
go run ./cmd/server
```

In another terminal, from this directory:

```sh
export WEBHOOK_SECRET=local-example-secret
go run .
```

With a freshly started receiver:

```text
First delivery: 204
Duplicate delivery: 204
Tampered delivery: 401
Processed orders: 1
Order: order-1001
```

Each sender run chooses a new delivery ID. Repeating a delivery within that run
has one effect; running the sender again creates a new event and increases the count.

The example's signature protocol is HMAC-SHA256 over `id.timestamp.rawBody`,
encoded as `v1=<lowercase hex>`. Headers are `x-webhook-id`,
`x-webhook-timestamp` (Unix seconds) and `x-webhook-signature`. All three signed
headers must be single values. Timestamps must be within five minutes of the
receiver's clock, including future timestamps. Both applications need the same
secret and reasonably synchronized clocks.

The generated receiver handles schema validation after authentication. Invalid
signatures or stale timestamps return 401; correctly signed invalid events return
400. Reusing an ID with different order data returns 409. Duplicate matching
order data returns 204. Request bodies are limited to 64 KiB (413).

Receipt IDs and processed orders live in memory; restarting the receiver clears
them. The deduplication check and effect run together (under a mutex).
This local example uses its own signing protocol; when connecting an external
provider, implement that provider's signature rules at the same boundary.

The server binds loopback on port `18800`. Override `WEBHOOK_API_PORT` and
`WEBHOOK_API_BASE_URL` together. Set `SDKGEN_BIN` for a particular CLI executable.
`cmd/server/main.go` contains the receiver; `main.go` is the sender. Shared signing code is in `internal/signature/`.
