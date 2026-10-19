import { createWriteStream } from "node:fs";
import { readFile } from "node:fs/promises";
import { pipeline } from "node:stream/promises";
import { createClient } from "./generated/api/index.js";

const api: ReturnType<typeof createClient> = createClient({
  baseURL: process.env.CONVERSION_API_BASE_URL ?? "http://127.0.0.1:18797",
});
const input: Buffer = await readFile("contacts.json");
const signal: AbortSignal = AbortSignal.timeout(10_000);
const file: Awaited<ReturnType<typeof api.$operations.convertContacts>> =
  await api.$operations.convertContacts({ body: input }, { signal });
await pipeline(file, createWriteStream("contacts.ndjson"), { signal });
console.log("Saved: contacts.ndjson");
process.stdout.write(await readFile("contacts.ndjson", "utf8"));
