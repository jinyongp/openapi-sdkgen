import { createWriteStream } from "node:fs";
import { readFile } from "node:fs/promises";
import { pipeline } from "node:stream/promises";
import { setTimeout as sleep } from "node:timers/promises";
import { createClient, type ComponentOutput } from "./generated/api/index.js";

type ReportJob = ComponentOutput<"ReportJob">;
const api: ReturnType<typeof createClient> = createClient({
  baseURL: process.env.REPORT_API_BASE_URL ?? "http://127.0.0.1:18795",
});
const abort: AbortController = new AbortController();
process.once("SIGINT", (): void => abort.abort());
const signal: AbortSignal = AbortSignal.any([
  abort.signal,
  AbortSignal.timeout(10_000),
]);

let job: ReportJob = await api.$operations.createReport(
  { body: { month: "2026-10" } },
  { signal },
);
console.log(`Report: ${job.status} (${job.processed}/${job.total})`);
while (job.status === "queued" || job.status === "running") {
  await sleep(100, undefined, { signal });
  const next: ReportJob = await api.$operations.getReport(
    { path: { reportID: job.id } },
    { signal },
  );
  if (next.status !== job.status || next.processed !== job.processed) {
    console.log(`Report: ${next.status} (${next.processed}/${next.total})`);
  }
  job = next;
}
if (job.status !== "completed")
  throw new Error(`Report ended as ${job.status}`);
const file: Awaited<ReturnType<typeof api.$operations.downloadReport>> =
  await api.$operations.downloadReport(
    { path: { reportID: job.id } },
    { signal },
  );
await pipeline(file, createWriteStream("monthly-sales.csv"), { signal });
console.log("Saved: monthly-sales.csv");
process.stdout.write(await readFile("monthly-sales.csv", "utf8"));

const unwanted: ReportJob = await api.$operations.createReport(
  { body: { month: "2026-10" } },
  { signal },
);
const cancelled: ReportJob = await api.$operations.cancelReport(
  { path: { reportID: unwanted.id } },
  { signal },
);
if (cancelled.status !== "cancelled")
  throw new Error("The second report was not cancelled");
console.log(`Second report: ${cancelled.status}`);
