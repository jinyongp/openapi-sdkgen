import { createHmac, timingSafeEqual } from "node:crypto";

export function sign(
  secret: string,
  id: string,
  timestamp: string,
  body: Uint8Array,
): string {
  const digest: string = createHmac("sha256", secret)
    .update(`${id}.${timestamp}.`)
    .update(body)
    .digest("hex");
  return `v1=${digest}`;
}
export function verify(
  secret: string,
  id: string,
  timestamp: string,
  signature: string,
  body: Uint8Array,
): boolean {
  if (
    !/^[A-Za-z0-9_-]{1,100}$/.test(id) ||
    !/^[0-9]{1,12}$/.test(timestamp) ||
    !/^v1=[0-9a-f]{64}$/.test(signature) ||
    Math.abs(Math.floor(Date.now() / 1000) - Number(timestamp)) > 300
  )
    return false;
  return timingSafeEqual(
    Buffer.from(signature.slice(3), "hex"),
    Buffer.from(sign(secret, id, timestamp, body).slice(3), "hex"),
  );
}
