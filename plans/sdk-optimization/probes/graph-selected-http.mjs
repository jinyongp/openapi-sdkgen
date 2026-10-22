import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import { pathToFileURL } from "node:url";

const [sdk, fixture] = process.argv.slice(2);
const { createClient } = await import(pathToFileURL(sdk).href);
const cases = JSON.parse(fs.readFileSync(fixture, "utf8"));
const seen = [];
const failures = [];
const server = http.createServer(async (request, response) => {
  try {
    const name = request.headers["x-case"];
    const expected = cases.find((row) => row.name === name);
    assert.ok(expected, "unexpected request case");
    const url = new URL(request.url, "http://loopback");
    assert.equal(request.method, expected.method);
    assert.equal(url.pathname, expected.path);
    assert.deepEqual(Object.fromEntries(url.searchParams), expected.query);
    assert.equal(request.headers.authorization, "Bearer fixture-token");
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const text = Buffer.concat(chunks).toString();
    assert.deepEqual(text ? JSON.parse(text) : null, expected.body);
    seen.push(name);
    response.writeHead(expected.status, { "Content-Type": "application/json" });
    response.end(expected.response === null ? "" : JSON.stringify(expected.response));
  } catch (error) {
    failures.push(String(error)); response.writeHead(500); response.end();
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
try {
  const client = createClient({ baseURL: `http://127.0.0.1:${server.address().port}`, authorization: "Bearer fixture-token" });
  for (const row of cases) {
    const options = { headers: { "X-Case": row.name } };
    let call;
    if (["requirements", "denied", "invalid"].includes(row.name)) {
      call = () => client.$routes["GET /users/{user-id}/authentication/requirements"]({ path: { "user-id": "u" }, query: { $select: ["perUserMfaState"] } }, options);
    } else if (row.name === "update") {
      call = () => client.$operations["users.authentication.UpdateRequirements"]({ path: { "user-id": "u" }, body: row.body }, options);
    } else if (row.name === "membership") {
      call = () => client.$operations["users.user.checkMemberGroups"]({ path: { "user-id": "u" }, body: row.body }, options);
    } else if (row.name === "mailtips") {
      call = () => client.$routes["POST /users/{user-id}/getMailTips"]({ path: { "user-id": "u" }, body: row.body }, options);
    } else {
      call = () => client.$operations["users.user.revokeSignInSessions"]({ path: { "user-id": "u" } }, options);
    }
    if (["denied", "invalid"].includes(row.name)) await assert.rejects(call);
    else {
      const result = await call();
      assert.deepEqual(result, row.response);
    }
  }
  assert.deepEqual(failures, []);
  assert.deepEqual(seen, cases.map((row) => row.name));
  assert.equal(client.$routes["GET /users"], undefined);
  assert.equal(client.$operations["users.user.GetUser"], undefined);
  console.log(JSON.stringify({ status: "pass", cases: seen, boundary: "real Node HTTP server and fetch" }));
} finally {
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}
