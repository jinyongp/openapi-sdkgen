import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const csv = "region,orders,revenueUSD\nAPAC,2,180.00\nEMEA,1,90.00\n";
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

export async function verifyReportClient(directory, output) {
  assert.ok(output.startsWith("Report: queued (0/3)\n"));
  assert.ok(output.includes("Report: completed (3/3)\n"));
  assert.ok(output.includes(`Saved: monthly-sales.csv\n${csv}`));
  assert.ok(output.endsWith("Second report: cancelled"));
  assert.equal(
    await readFile(resolve(directory, "monthly-sales.csv"), "utf8"),
    csv,
  );
}

export async function verifyReportServer(baseURL) {
  const call = async (path, method = "GET", body) => {
    const response = await fetch(`${baseURL}${path}`, {
      method,
      ...(body === undefined
        ? {}
        : { body, headers: { "content-type": "application/json" } }),
      signal: AbortSignal.timeout(5_000),
    });
    const text = await response.text();
    return {
      status: response.status,
      headers: response.headers,
      text,
      data: response.headers.get("content-type")?.includes("application/json")
        ? JSON.parse(text)
        : undefined,
    };
  };
  const create = async (month = "2026-10") => {
    const response = await call("/reports", "POST", JSON.stringify({ month }));
    assert.equal(response.status, 202);
    assert.equal(response.data.status, "queued");
    assert.equal(response.data.processed, 0);
    return response.data;
  };
  const wait = async (id, predicate) => {
    for (let attempt = 0; attempt < 50; attempt++) {
      const response = await call(`/reports/${id}`);
      assert.equal(response.status, 200);
      if (predicate(response.data)) return response.data;
      await sleep(20);
    }
    assert.fail("Report did not reach the required state");
  };
  for (const body of ["{", '{"month":"2026-13"}', '{"month":"2026-10"}{}']) {
    assert.equal((await call("/reports", "POST", body)).status, 400);
  }
  for (const [path, method] of [
    ["/reports/missing", "GET"],
    ["/reports/missing/file", "GET"],
    ["/reports/missing/cancel", "POST"],
  ]) {
    assert.equal((await call(path, method)).status, 404);
  }

  const cancelled = await create();
  assert.equal(cancelled.total, 3);
  assert.equal((await call(`/reports/${cancelled.id}/file`)).status, 409);
  const running = await wait(cancelled.id, (job) => job.status === "running");
  assert.ok(running.processed > 0 && running.processed < running.total);
  const stopped = await call(`/reports/${cancelled.id}/cancel`, "POST");
  assert.equal(stopped.status, 200);
  assert.equal(stopped.data.status, "cancelled");
  assert.deepEqual(
    (await call(`/reports/${cancelled.id}/cancel`, "POST")).data,
    stopped.data,
  );
  await sleep(700);
  assert.deepEqual(
    (await call(`/reports/${cancelled.id}`)).data,
    stopped.data,
    "Cancelled workers must not advance or complete",
  );
  assert.equal((await call(`/reports/${cancelled.id}/file`)).status, 409);

  const completed = await create();
  const final = await wait(completed.id, (job) => job.status === "completed");
  assert.equal(final.processed, 3);
  assert.equal(
    (await call(`/reports/${completed.id}/cancel`, "POST")).status,
    409,
  );
  const report = await call(`/reports/${completed.id}/file`);
  assert.equal(report.status, 200);
  assert.equal(report.headers.get("content-type"), "application/octet-stream");
  assert.equal(report.text, csv);

  const september = await create("2026-09");
  const septemberFinal = await wait(
    september.id,
    (job) => job.status === "completed",
  );
  assert.equal(septemberFinal.total, 1);
  assert.equal(
    (await call(`/reports/${september.id}/file`)).text,
    "region,orders,revenueUSD\nAMER,1,99.00\n",
  );

  const empty = await create("2026-08");
  const emptyFinal = await wait(empty.id, (job) => job.status === "completed");
  assert.equal(emptyFinal.total, 0);
  assert.equal(emptyFinal.processed, 0);
  assert.equal(
    (await call(`/reports/${empty.id}/file`)).text,
    "region,orders,revenueUSD\n",
  );
}
