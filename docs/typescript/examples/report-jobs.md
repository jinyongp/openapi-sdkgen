# Generate a background report

Request a monthly sales report, watch its progress and download the CSV when it
finishes. [Download the example](/downloads/typescript-report-jobs-app.zip) for a
separate HTTP server, generated client, order data and run instructions.

The server aggregates `orders.json` by region. The client sends the month and
receives a job ID; it does not compute the report itself.

## Submit and wait {#submit}

With the example server running, create a client for `http://127.0.0.1:18795`.
The runnable client supplies a cancellation signal with a ten-second deadline.

```ts
import { setTimeout as sleep } from "node:timers/promises";

let job = await api.$operations.createReport(
  { body: { month: "2026-10" } },
  { signal },
);
while (job.status === "queued" || job.status === "running") {
  await sleep(100, undefined, { signal });
  job = await api.$operations.getReport(
    { path: { reportID: job.id } },
    { signal },
  );
}
if (job.status !== "completed")
  throw new Error(`Report ended as ${job.status}`);
console.log(`Completed: ${job.processed}/${job.total} orders`);
```

```text
Completed: 3/3 orders
```

## Save the result {#download}

The download operation returns a byte stream. Pipe it into a local file:

```ts
import { createWriteStream } from "node:fs";
import { pipeline } from "node:stream/promises";

const file = await api.$operations.downloadReport(
  { path: { reportID: job.id } },
  { signal },
);
await pipeline(file, createWriteStream("monthly-sales.csv"), { signal });
```

The file contains October's totals. The September order is excluded:

```text
region,orders,revenueUSD
APAC,2,180.00
EMEA,1,90.00
```

## Cancel a job {#cancel}

Create a second job, then cancel it before it finishes:

```ts
const second = await api.$operations.createReport({
  body: { month: "2026-10" },
});
const cancelled = await api.$operations.cancelReport({
  path: { reportID: second.id },
});
console.log(cancelled.status);
```

```text
cancelled
```

Stopping polling or aborting an HTTP request does not cancel the background job.
Use the cancel operation explicitly. An unfinished or cancelled report has no
download (HTTP 409); a completed job cannot be cancelled (HTTP 409). Repeating a
cancellation is safe. Unknown job IDs return HTTP 404.

The local server keeps jobs in memory and pauses between records so progress can
be observed. Restarting it clears job state. Edit `orders.json` before restarting
to compute different reports.
