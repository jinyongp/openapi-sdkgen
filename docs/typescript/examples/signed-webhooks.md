# Receive signed webhooks

A separate sender signs an order event. The receiver verifies its signature, then
uses a generated webhook handler to validate the event and record the order.
[Download the example](/downloads/typescript-signed-webhook-app.zip) for both programs and run instructions.

## Sign and send {#send}

The sender prepares raw `body` bytes, a delivery `id` and a Unix-seconds `timestamp`.
Set the same `WEBHOOK_SECRET` in both terminals.
The sender uses `sign` from `src/signature.ts`.

```ts
const signature = sign(secret, id, timestamp, body);
const response = await fetch(`${baseURL}/hooks/orders/paid`, {
  method: "POST",
  headers: {
    "content-type": "application/json",
    "x-webhook-id": id,
    "x-webhook-timestamp": timestamp,
    "x-webhook-signature": signature,
  },
  body: new Uint8Array(body),
});
console.log(response.status);
```

Running the complete example against a freshly started receiver produces:

```text
First delivery: 204
Duplicate delivery: 204
Tampered delivery: 401
Processed orders: 1
Order: order-1001
```

The last two lines come from the generated SDK calling `GET /fulfillments`.
Sending the same delivery twice records the order once. Each new sender run uses a new delivery ID.

## Verification and deduplication {#verify}

This example signs `id.timestamp.rawBody` with HMAC-SHA256, encoded as
`v1=<lowercase hex>`. The receiver compares the original bytes and accepts
only timestamps within five minutes of its clock, including future timestamps.
The generated handler validates JSON only after signature verification.

Bad signatures and stale timestamps return 401. Correctly signed events that
violate the schema return 400. Matching ID/data retries return 204 without
reprocessing; reusing an ID for different order data returns 409. Bodies above
64 KiB return 413.

The deduplication check and order recording happen together. Receipts are stored
in memory and cleared on receiver restart. Use the external provider's signature
rules when integrating a real provider.
