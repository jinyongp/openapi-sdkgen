import { randomUUID } from "node:crypto";
import { createClient } from "./generated/api/index.js";
import { sign } from "./signature.js";

const secret: string | undefined = process.env.WEBHOOK_SECRET;
if (!secret) throw new Error("Set WEBHOOK_SECRET on the receiver and sender.");
const baseURL: string =
  process.env.WEBHOOK_API_BASE_URL ?? "http://127.0.0.1:18799";
const api: ReturnType<typeof createClient> = createClient({ baseURL });
const id: string = randomUUID();
const timestamp: string = String(Math.floor(Date.now() / 1000));
const body: Buffer = Buffer.from(
  JSON.stringify({ orderID: "order-1001", amountCents: 27000 }),
);

async function deliver(
  payload: Uint8Array,
  signature: string,
): Promise<number> {
  const response: Response = await fetch(`${baseURL}/hooks/orders/paid`, {
    method: "POST",
    body: new Uint8Array(payload),
    headers: {
      "content-type": "application/json",
      "x-webhook-id": id,
      "x-webhook-timestamp": timestamp,
      "x-webhook-signature": signature,
    },
    signal: AbortSignal.timeout(10_000),
  });
  await response.arrayBuffer();
  return response.status;
}
const signature: string = sign(secret, id, timestamp, body);
console.log(`First delivery: ${await deliver(body, signature)}`);
console.log(`Duplicate delivery: ${await deliver(body, signature)}`);
const changed: Buffer = Buffer.from(
  JSON.stringify({ orderID: "order-1001", amountCents: 1 }),
);
console.log(`Tampered delivery: ${await deliver(changed, signature)}`);
const result: Awaited<ReturnType<typeof api.$operations.getFulfillments>> =
  await api.$operations.getFulfillments({ timeoutMS: 10_000 });
console.log(`Processed orders: ${result.processed}`);
console.log(`Order: ${result.orders.at(-1)}`);
