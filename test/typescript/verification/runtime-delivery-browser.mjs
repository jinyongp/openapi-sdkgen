// Local native-ESM witness for runtime-delivery-check artifacts. No external API access.
import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { brotliCompressSync, gzipSync, constants } from "node:zlib";
import { exerciseRuntimeDelivery } from "./runtime-delivery-fixture.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const latest = JSON.parse(
  fs.readFileSync(path.join(root, ".tmp/runtime-delivery/latest.json"), "utf8"),
);
assert.equal(
  latest.status,
  "pass",
  "Run runtime-delivery-check successfully before browser serving",
);
const reportFile = path.resolve(root, latest.report);
const runDir = path.dirname(reportFile);
assert(runDir.startsWith(path.join(root, ".tmp/runtime-delivery") + path.sep));
const report = JSON.parse(fs.readFileSync(reportFile, "utf8"));
const browserRun = randomUUID();
const browserDir = path.join(runDir, "browser", browserRun);
fs.mkdirSync(browserDir, { recursive: true });
const logFile = path.join(browserDir, "requests.jsonl");
const cases = [];
for (let repetition = 0; repetition < 11; repetition++) {
  for (const names of [
    ["get"],
    ["get", "post"],
    ["get", "xml"],
    ["get", "stream"],
    ["get", "xml", "stream"],
  ]) {
    for (const kind of repetition % 2 === 0
      ? ["baseline", "candidate"]
      : ["candidate", "baseline"]) {
      cases.push({ id: `${browserRun}-${cases.length}`, kind, names, repetition });
    }
  }
}
const byID = new Map(cases.map((entry) => [entry.id, entry]));
const encode = (body, request) => {
  const accepted = request.headers["accept-encoding"] ?? "";
  if (accepted.includes("br"))
    return {
      body: brotliCompressSync(body, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }),
      encoding: "br",
    };
  if (accepted.includes("gzip")) return { body: gzipSync(body, { level: 6 }), encoding: "gzip" };
  return { body, encoding: "identity" };
};
function page(entry) {
  return `<!doctype html><meta charset="utf-8"><title>Runtime native ESM case</title><pre id="result">Running</pre><script type="module">
const entry = ${JSON.stringify(entry)};
const started = performance.now();
try {
  const {createAPIs} = await import('/assets/' + entry.id + '/' + entry.kind + '/' + entry.names.join('-') + '/entry.js');
  const exercise = ${exerciseRuntimeDelivery.toString()};
  const semantics = await exercise(createAPIs, entry.names, location.origin + '/api/' + entry.id);
  const assets = performance.getEntriesByType('resource').filter(r=>r.name.includes('/assets/'+entry.id+'/'));
  const result={...entry,pass:true,semantics,elapsedMS:performance.now()-started,encodedBodySize:assets.reduce((n,r)=>n+r.encodedBodySize,0),transferSize:assets.reduce((n,r)=>n+r.transferSize,0),requests:assets.length,paths:assets.map(r=>new URL(r.name).pathname)};
  document.querySelector('#result').textContent=JSON.stringify(result);
  console.log('RUNTIME_BROWSER_CASE '+JSON.stringify(result));
  parent.postMessage({type:'runtime-result',result},location.origin);
} catch(error) {
  const result={...entry,pass:false,error:String(error?.stack??error)};
  document.querySelector('#result').textContent=JSON.stringify(result);
  console.error('RUNTIME_BROWSER_CASE '+JSON.stringify(result));
  parent.postMessage({type:'runtime-result',result},location.origin);
}
</script>`;
}
const suite = `<!doctype html><meta charset="utf-8"><title>Runtime native ESM suite</title><h1>Runtime native ESM</h1><pre id="summary">Starting</pre><iframe id="case" style="width:100%;height:260px"></iframe><script type="module">
const cases=${JSON.stringify(cases)};const results=[];const frame=document.querySelector('#case');let index=0;
const summarize=()=>{const groups=new Map();for(const r of results){const key=r.kind+':'+r.names.join('+');let group=groups.get(key);if(!group){group={kind:r.kind,names:r.names,passed:0,bodyBytes:[],requests:[],elapsedMS:[],semanticsPassed:0,postSamples:[],twoClientChecks:0,errors:[]};groups.set(key,group);}if(r.pass){group.passed++;if(r.semantics?.apiTransport==='injected-fetch'){group.semanticsPassed++;group.postSamples.push(r.semantics.requests.filter(x=>x.method==='POST').length);if(r.semantics.checks.includes('two-client-isolation'))group.twoClientChecks++;}group.bodyBytes.push(r.encodedBodySize);group.requests.push(r.requests);group.elapsedMS.push(r.elapsedMS);}else group.errors.push(r.error);}return [...groups.values()];};
window.addEventListener('message',event=>{if(event.origin!==location.origin||event.source!==frame.contentWindow||event.data?.type!=='runtime-result')return;const result=event.data.result;if(result.id!==cases[index]?.id)return;results.push(result);index++;document.querySelector('#summary').textContent=JSON.stringify({completed:results.length,total:cases.length,failed:results.filter(r=>!r.pass).length},null,2);if(index<cases.length)frame.src='/case/'+cases[index].id;else {for(const group of summarize())console.log('RUNTIME_BROWSER_GROUP '+JSON.stringify(group));console.log('RUNTIME_BROWSER_SUMMARY '+JSON.stringify({browserRun:${JSON.stringify(browserRun)},runID:${JSON.stringify(latest.runID)},cases:results.length,passed:results.filter(r=>r.pass).length}));}});
frame.src='/case/'+cases[0].id;
</script>`;
const server = http.createServer((request, response) => {
  const url = new URL(request.url ?? "/", "http://localhost");
  let body;
  let type;
  let caseID;
  let category = "harness";
  if (url.pathname === "/" && request.method === "GET") {
    body = Buffer.from(suite);
    type = "text/html; charset=utf-8";
  } else if (url.pathname.startsWith("/case/") && request.method === "GET") {
    const entry = byID.get(url.pathname.slice(6));
    if (entry) {
      body = Buffer.from(page(entry));
      type = "text/html; charset=utf-8";
      caseID = entry.id;
    }
  } else if (url.pathname.startsWith("/assets/") && request.method === "GET") {
    const parts = url.pathname.split("/");
    caseID = parts[2];
    const entry = byID.get(caseID);
    if (entry && parts[3] === entry.kind && url.pathname.endsWith(".js")) {
      const file = path.resolve(runDir, ...parts.slice(3));
      if (
        file.startsWith(path.join(runDir, entry.kind) + path.sep) &&
        fs.existsSync(file) &&
        fs.statSync(file).isFile()
      ) {
        body = fs.readFileSync(file);
        type = "text/javascript; charset=utf-8";
        category = "sdk";
      }
    }
  } else if (url.pathname.startsWith("/api/")) {
    const parts = url.pathname.split("/");
    caseID = parts[2];
    if (byID.has(caseID) && ["GET", "POST"].includes(request.method)) {
      category = "api";
      const route = parts.slice(3).join("/");
      if (route === "events") {
        body = Buffer.from('{"n":1}\n{"n":2}\n');
        type = "application/x-ndjson";
      } else if (route === "xml") {
        body = Buffer.from("<todo><title>XML</title></todo>");
        type = "application/xml";
      } else {
        body = Buffer.from('{"todo_id":"todo-1","title":"JSON"}');
        type = "application/json";
      }
      request.resume();
    }
  }
  if (body === undefined) {
    response.writeHead(404, { "content-type": "text/plain" });
    response.end("Not found");
    return;
  }
  const packed = encode(body, request);
  const headers = {
    "content-type": type,
    "content-length": packed.body.length,
    "x-content-type-options": "nosniff",
    "cache-control": category === "sdk" ? "public, max-age=31536000, immutable" : "no-store",
    vary: "Accept-Encoding",
  };
  if (packed.encoding !== "identity") headers["content-encoding"] = packed.encoding;
  response.writeHead(200, headers);
  response.end(packed.body);
  fs.appendFileSync(
    logFile,
    JSON.stringify({
      caseID,
      category,
      method: request.method,
      path: url.pathname,
      encoding: packed.encoding,
      bodyBytes: packed.body.length,
      rawBytes: body.length,
    }) + "\n",
  );
});
server.listen(0, "127.0.0.1", () => {
  const origin = `http://127.0.0.1:${server.address().port}`;
  const manifest = {
    apiTransport: "injected-fetch",
    readOnlyMode: false,
    excludedWorkloads: [],
    runID: latest.runID,
    browserRun,
    origin,
    report: path.relative(root, reportFile),
    directory: path.relative(root, browserDir),
    cases,
  };
  fs.writeFileSync(
    path.join(browserDir, "manifest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
  fs.writeFileSync(
    path.join(root, ".tmp/runtime-delivery/browser-latest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
  console.log(
    JSON.stringify({
      ready: true,
      origin,
      browserRun,
      runID: latest.runID,
      cases: cases.length,
      log: path.relative(root, logFile),
    }),
  );
});
const stop = () => server.close(() => process.exit(0));
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
