import assert from "node:assert/strict";
import { createHmac, randomUUID } from "node:crypto";

export const webhookSecret = "local-example-secret";
export function verifySignedWebhookClient(output) {
  assert.equal(
    output,
    "First delivery: 204\nDuplicate delivery: 204\nTampered delivery: 401\nProcessed orders: 1\nOrder: order-1001",
  );
}
export async function verifySignedWebhookServer(baseURL) {
  const state = async () => {
    const response = await fetch(`${baseURL}/fulfillments`, {
      signal: AbortSignal.timeout(5_000),
    });
    assert.equal(response.status, 200);
    return response.json();
  };
  const timestamp = String(Math.floor(Date.now() / 1000));
  const body = '{"orderID":"order-parallel","amountCents":1250}';
  const headers = (id, payload = body, time = timestamp) => ({
    "content-type": "application/json",
    "x-webhook-id": id,
    "x-webhook-timestamp": time,
    "x-webhook-signature": `v1=${createHmac("sha256", webhookSecret).update(`${id}.${time}.${payload}`).digest("hex")}`,
  });
  const deliver = async (payload, signedHeaders) => {
    const response = await fetch(`${baseURL}/hooks/orders/paid`, {
      method: "POST",
      headers: signedHeaders,
      body: payload,
      signal: AbortSignal.timeout(5_000),
    });
    await response.arrayBuffer();
    return response.status;
  };
  const before = await state();
  const id = randomUUID();
  const signed = headers(id);
  assert.equal(
    await deliver(body + " ", signed),
    401,
    "Verify exact bytes, not reserialized JSON",
  );
  assert.equal(
    await deliver(body, { ...signed, "x-webhook-id": randomUUID() }),
    401,
  );
  assert.equal(
    await deliver(body, {
      ...signed,
      "x-webhook-timestamp": String(Number(timestamp) - 1),
    }),
    401,
  );
  assert.equal(
    await deliver(body, { ...signed, "x-webhook-signature": "v1=bad" }),
    401,
  );
  assert.equal(
    await deliver("{", {}),
    401,
    "Authentication precedes JSON decoding",
  );
  for (const offset of [-3600, 3600]) {
    const time = String(Number(timestamp) + offset);
    assert.equal(await deliver(body, headers(id, body, time)), 401);
  }
  assert.deepEqual(
    await state(),
    before,
    "Rejected signatures must not change state",
  );

  assert.deepEqual(
    await Promise.all(Array.from({ length: 5 }, () => deliver(body, signed))),
    [204, 204, 204, 204, 204],
  );
  const after = await state();
  assert.equal(after.processed, before.processed + 1);
  assert.deepEqual(after.orders, [...before.orders, "order-parallel"]);
  const reformatted = JSON.stringify(JSON.parse(body), null, 2);
  assert.equal(
    await deliver(
      reformatted,
      headers(id, reformatted, String(Number(timestamp) - 30)),
    ),
    204,
    "A correctly re-signed retry of the same data must remain idempotent",
  );
  assert.deepEqual(await state(), after);
  const changed = '{"orderID":"order-different","amountCents":1250}';
  assert.equal(await deliver(changed, headers(id, changed)), 409);
  assert.deepEqual(await state(), after);

  const repairedID = randomUUID();
  for (const invalid of [
    "{",
    '{"orderID":"order-repaired"}',
    '{"orderID":"order-repaired","amountCents":0}',
  ]) {
    assert.equal(await deliver(invalid, headers(repairedID, invalid)), 400);
  }
  const repaired = '{"orderID":"order-repaired","amountCents":100}';
  assert.equal(
    await deliver(repaired, headers(repairedID, repaired)),
    204,
    "Invalid bodies must not reserve delivery IDs",
  );
  const large = " ".repeat(65 * 1024);
  assert.equal(await deliver(large, headers(randomUUID(), large)), 413);
  const final = await state();
  assert.equal(final.processed, before.processed + 2);
  assert.deepEqual(final.orders, [
    ...before.orders,
    "order-parallel",
    "order-repaired",
  ]);
}
