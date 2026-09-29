// Measure sequential runtime capabilities and real HTTP cache reuse independently of API traffic.
import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { randomUUID, createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { brotliCompressSync, constants } from "node:zlib";
import { exerciseRuntimeDelivery } from "./runtime-delivery-fixture.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const reportPath = path.resolve(
  root,
  process.argv[2] ??
    JSON.parse(fs.readFileSync(path.join(root, ".tmp/runtime-delivery/latest.json"), "utf8"))
      .report,
);
assert(reportPath.startsWith(path.join(root, ".tmp/runtime-delivery") + path.sep));
const runtime = JSON.parse(fs.readFileSync(reportPath, "utf8"));
assert.equal(runtime.status, "pass");
const runDirectory = path.dirname(reportPath);
const sessionRun = randomUUID();
const directory = path.join(runDirectory, "sessions", sessionRun);
fs.mkdirSync(directory, { recursive: true });
const log = path.join(directory, "requests.jsonl");
const phases = ["cold", "revisit", "new-url-version", "retained-old-url"];
const workloads = [["get"], ["get", "xml"], ["get", "xml", "stream"], ["get", "post"]];
const assets = new Map();
for (const workload of runtime.workloads) {
  for (const name of workload.native.inventory) {
    if (assets.has(name)) continue;
    const filename = path.resolve(runDirectory, name);
    assert(filename.startsWith(runDirectory + path.sep));
    const bytes = fs.readFileSync(filename);
    assets.set(name, {
      bytes,
      packed: brotliCompressSync(bytes, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }),
      sha256: createHash("sha256").update(bytes).digest("hex"),
    });
  }
}
const cases = [];
for (let repetition = 0; repetition < 11; repetition++) {
  for (const kind of repetition % 2 === 0 ? ["baseline", "candidate"] : ["candidate", "baseline"]) {
    for (const phase of phases) {
      cases.push({
        id: String(cases.length),
        repetition,
        kind,
        phase,
        version: phase === "new-url-version" ? "v2" : "v1",
      });
    }
  }
}
const byID = new Map(cases.map((entry) => [entry.id, entry]));
function page(entry) {
  const prefix = `/assets/${sessionRun}/${entry.repetition}/${entry.version}/`;
  const inventories = workloads.map((names) => {
    const build = runtime.workloads.find(
      (row) => row.kind === entry.kind && row.names.join("+") === names.join("+"),
    );
    assert(build);
    return { names, paths: [...build.native.inventory].sort() };
  });
  return `<!doctype html><meta charset="utf-8"><title>Runtime session case</title><pre id="result">Running</pre><script type="module">
const entry=${JSON.stringify(entry)}, prefix=${JSON.stringify(prefix)}, inventory=${JSON.stringify(inventories)};
const started=performance.now();
try {
  const exercise=${exerciseRuntimeDelivery.toString()};
  const stages=[]; const all=new Set(); let previous=new Set();
  const resources=()=>performance.getEntriesByType('resource').filter(row=>new URL(row.name).pathname.startsWith(prefix));
  if(resources().length)throw Error('SDK loaded before selection');
  for (const step of inventory) {
    const stageStarted=performance.now();
    const url=prefix+entry.kind+'/'+step.names.join('-')+'/entry.js';
    const module=await import(url);
    const semantic=await exercise(module.createAPIs,step.names,location.origin+'/api/'+entry.id);
    if(!semantic.checks.includes('two-client-isolation'))throw Error('Missing two-client test');
    if(step.names.includes('post')&&!semantic.checks.includes('post-invalid-before-fetch'))throw Error('Missing POST validation');
    for(const name of step.paths)all.add(prefix+name);
    const rows=resources(); const paths=rows.map(row=>new URL(row.name).pathname);
    if(JSON.stringify([...paths].sort())!==JSON.stringify([...all].sort()))throw Error('Unexpected cumulative module graph: '+JSON.stringify(paths));
    const delta=rows.filter(row=>!previous.has(new URL(row.name).pathname));
    const beforeRepeat=rows.length;
    if(await import(url)!==module)throw Error('Module identity changed');
    if(resources().length!==beforeRepeat)throw Error('Repeated import fetched again in the same realm');
    stages.push({names:step.names,files:delta.length,body:delta.reduce((n,r)=>n+r.encodedBodySize,0),transfer:delta.reduce((n,r)=>n+r.transferSize,0),elapsedMS:performance.now()-stageStarted});
    previous=new Set(paths);
  }
  const result={...entry,pass:true,stages,elapsedMS:performance.now()-started};
  console.log('RUNTIME_SESSION_CASE '+JSON.stringify(result));
  document.querySelector('#result').textContent=JSON.stringify(result);
  parent.postMessage({type:'runtime-session',result},location.origin);
} catch(error) {
  const result={...entry,pass:false,error:String(error?.stack??error)};
  console.error('RUNTIME_SESSION_CASE '+JSON.stringify(result));
  document.querySelector('#result').textContent=JSON.stringify(result);
  parent.postMessage({type:'runtime-session',result},location.origin);
}
</script>`;
}
const suite = `<!doctype html><meta charset="utf-8"><title>Runtime session cache suite</title><h1>Runtime sessions</h1><pre id="summary">Starting</pre><iframe id="case"></iframe><script type="module">
const cases=${JSON.stringify(cases)}, frame=document.querySelector('#case'), results=[];let index=0;
window.addEventListener('message',event=>{
 if(event.origin!==location.origin||event.source!==frame.contentWindow||event.data?.type!=='runtime-session')return;
 const result=event.data.result;if(result.id!==cases[index]?.id)return;
 results.push(result);index++;document.querySelector('#summary').textContent=JSON.stringify({completed:index,total:cases.length,failed:results.filter(r=>!r.pass).length});
 if(index<cases.length)frame.src='/case/'+cases[index].id;
 else{
  const groups=new Map();for(const r of results){const key=r.kind+':'+r.phase;let g=groups.get(key);if(!g){g={kind:r.kind,phase:r.phase,passed:0,signatures:[],elapsedMS:[],errors:[]};groups.set(key,g);}if(r.pass){g.passed++;g.elapsedMS.push(r.elapsedMS);g.signatures.push(r.stages.map(s=>[s.files,s.body,s.transfer]).map(s=>s.join('/')).join(','));}else g.errors.push(r.error);}
  for(const g of groups.values())console.log('RUNTIME_SESSION_GROUP '+JSON.stringify(g));
  console.log('RUNTIME_SESSION_SUMMARY '+JSON.stringify({runID:${JSON.stringify(runtime.runID)},sessionRun:${JSON.stringify(sessionRun)},cases:results.length,passed:results.filter(r=>r.pass).length}));
 }
});frame.src='/case/'+cases[0].id;
</script>`;
let currentCase;
const server = http.createServer((request, response) => {
  const url = new URL(request.url ?? "/", "http://localhost");
  let data,
    type,
    category = "harness",
    asset;
  if (request.method === "GET" && url.pathname === "/") {
    data = Buffer.from(suite);
    type = "text/html; charset=utf-8";
  } else if (request.method === "GET" && url.pathname.startsWith("/case/")) {
    const entry = byID.get(url.pathname.slice(6));
    if (entry) {
      currentCase = entry;
      data = Buffer.from(page(entry));
      type = "text/html; charset=utf-8";
    }
  } else if (request.method === "GET" && url.pathname.startsWith("/assets/")) {
    const parts = url.pathname.split("/");
    if (
      currentCase &&
      parts[2] === sessionRun &&
      parts[3] === String(currentCase.repetition) &&
      parts[4] === currentCase.version
    ) {
      asset = parts.slice(5).join("/");
      const value = assets.get(asset);
      if (value && asset.startsWith(currentCase.kind + "/")) {
        data = value.packed;
        type = "text/javascript; charset=utf-8";
        category = "sdk";
      }
    }
  }
  const status = data === undefined ? 404 : 200;
  data ??= Buffer.from("Not found");
  type ??= "text/plain";
  const headers = {
    "content-type": type,
    "content-length": data.length,
    "x-content-type-options": "nosniff",
    "cache-control": category === "sdk" ? "public, max-age=31536000, immutable" : "no-store",
  };
  if (category === "sdk") headers["content-encoding"] = "br";
  response.writeHead(status, headers);
  response.end(data);
  fs.appendFileSync(
    log,
    JSON.stringify({
      caseID: currentCase?.id,
      category,
      method: request.method,
      path: url.pathname,
      asset,
      status,
      bodyBytes: data.length,
    }) + "\n",
  );
});
server.listen(0, "127.0.0.1", () => {
  const origin = `http://127.0.0.1:${server.address().port}`;
  const manifest = {
    runID: runtime.runID,
    sessionRun,
    origin,
    report: path.relative(root, reportPath),
    directory: path.relative(root, directory),
    cases,
    workloads,
    assets: Object.fromEntries(
      [...assets].map(([name, v]) => [
        name,
        { bytes: v.bytes.length, brotli5: v.packed.length, sha256: v.sha256 },
      ]),
    ),
    scope:
      "Physical runtime sessions. v2 changes only the asset URL prefix; it is not a second generator build or a cross-generation ABI proof.",
  };
  fs.writeFileSync(path.join(directory, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
  fs.writeFileSync(
    path.join(root, ".tmp/runtime-delivery/session-latest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
  console.log(
    JSON.stringify({
      ready: true,
      origin,
      runID: runtime.runID,
      sessionRun,
      cases: cases.length,
      directory: manifest.directory,
    }),
  );
});
const stop = () => server.close(() => process.exit(0));
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
