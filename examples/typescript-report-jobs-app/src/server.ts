import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import type { ComponentOutput } from "./generated/api/index.js";

type ReportJob = ComponentOutput<"ReportJob">;

interface Order {
  readonly date: string;
  readonly region: string;
  readonly amountCents: number;
}
interface RegionTotal {
  orders: number;
  amountCents: number;
}
interface Work {
  job: ReportJob;
  readonly orders: readonly Order[];
  readonly regions: Map<string, RegionTotal>;
  timer?: ReturnType<typeof setTimeout>;
  csv?: string;
}

const input: unknown = JSON.parse(await readFile("orders.json", "utf8"));
if (
  !Array.isArray(input) ||
  !input.every(
    (value: unknown): value is Order =>
      typeof value === "object" &&
      value !== null &&
      "date" in value &&
      typeof value.date === "string" &&
      /^\d{4}-\d{2}-\d{2}$/.test(value.date) &&
      "region" in value &&
      typeof value.region === "string" &&
      /^[A-Z]+$/.test(value.region) &&
      "amountCents" in value &&
      typeof value.amountCents === "number" &&
      Number.isSafeInteger(value.amountCents) &&
      value.amountCents >= 0,
  )
) {
  throw new Error(
    "orders.json must contain dated orders with region and non-negative integer cents",
  );
}
const orders: readonly Order[] = input;
const jobs: Map<string, Work> = new Map();

function advance(work: Work): void {
  if (work.job.status === "cancelled") return;
  work.job = { ...work.job, status: "running" };
  const order: Order | undefined = work.orders[work.job.processed];
  if (order) {
    const total: RegionTotal = work.regions.get(order.region) ?? {
      orders: 0,
      amountCents: 0,
    };
    total.orders++;
    total.amountCents += order.amountCents;
    work.regions.set(order.region, total);
    work.job = { ...work.job, processed: work.job.processed + 1 };
  }
  if (work.job.processed === work.job.total) {
    const rows: string[] = ["region,orders,revenueUSD"];
    for (const [region, total] of [...work.regions].sort(
      ([left], [right]): number => left.localeCompare(right),
    )) {
      rows.push(
        `${region},${total.orders},${(total.amountCents / 100).toFixed(2)}`,
      );
    }
    work.csv = rows.join("\n") + "\n";
    work.job = { ...work.job, status: "completed" };
    return;
  }
  // Pause between records so the client can observe progress and cancel work.
  work.timer = setTimeout((): void => advance(work), 200);
}

function json(response: ServerResponse, status: number, body: unknown): void {
  response.writeHead(status, { "content-type": "application/json" });
  response.end(JSON.stringify(body));
}

async function handle(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  const path: string = new URL(request.url ?? "/", "http://localhost").pathname;
  if (request.method === "POST" && path === "/reports") {
    let text: string = "";
    for await (const chunk of request) {
      text += chunk;
      if (Buffer.byteLength(text) > 4096) {
        json(response, 400, { message: "Report input is too large" });
        return;
      }
    }
    let body: unknown;
    try {
      body = JSON.parse(text);
    } catch {
      json(response, 400, { message: "Expected a JSON month" });
      return;
    }
    if (
      typeof body !== "object" ||
      body === null ||
      !("month" in body) ||
      typeof body.month !== "string" ||
      !/^\d{4}-(0[1-9]|1[0-2])$/.test(body.month)
    ) {
      json(response, 400, { message: "Use a month such as 2026-10" });
      return;
    }
    const month: string = body.month;
    const selected: readonly Order[] = orders.filter((order: Order): boolean =>
      order.date.startsWith(month),
    );
    const work: Work = {
      job: {
        id: randomUUID(),
        month: body.month,
        status: "queued",
        processed: 0,
        total: selected.length,
      },
      orders: selected,
      regions: new Map(),
    };
    jobs.set(work.job.id, work);
    work.timer = setTimeout((): void => advance(work), 200);
    json(response, 202, work.job);
    return;
  }
  const match: RegExpMatchArray | null = path.match(
    /^\/reports\/([^/]+)(?:\/(file|cancel))?$/,
  );
  const work: Work | undefined = match ? jobs.get(match[1]) : undefined;
  if (!work || !match) {
    json(response, 404, { message: "Report not found" });
    return;
  }
  if (request.method === "GET" && !match[2]) {
    json(response, 200, work.job);
    return;
  }
  if (request.method === "POST" && match[2] === "cancel") {
    if (work.job.status === "completed") {
      json(response, 409, { message: "Completed reports cannot be cancelled" });
      return;
    }
    clearTimeout(work.timer);
    work.job = { ...work.job, status: "cancelled" };
    json(response, 200, work.job);
    return;
  }
  if (request.method === "GET" && match[2] === "file") {
    if (work.job.status !== "completed") {
      json(response, 409, { message: "Report is not completed" });
      return;
    }
    response.writeHead(200, {
      "content-type": "application/octet-stream",
      "content-disposition": 'attachment; filename="monthly-sales.csv"',
    });
    response.end(work.csv);
    return;
  }
  json(response, 404, { message: "Route not found" });
}

const server: ReturnType<typeof createServer> = createServer(
  (request: IncomingMessage, response: ServerResponse): void => {
    void handle(request, response).catch((): void => {
      if (!response.headersSent)
        json(response, 500, { message: "Report request failed" });
      else response.destroy();
    });
  },
);
server.listen(
  Number(process.env.REPORT_API_PORT ?? 18795),
  "127.0.0.1",
  (): void => {
    console.log("Report API listening");
  },
);
