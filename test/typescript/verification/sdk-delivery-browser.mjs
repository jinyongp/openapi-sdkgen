// Real generated SDK assets over HTTP; API semantics use browser-owned injected fetch.
import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { randomUUID, createHash } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { brotliCompressSync, constants } from "node:zlib";
import { exerciseGeneratedSDK } from "./sdk-delivery-fixture.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const parser = createRequire(
  path.join(root, "test/typescript/node_modules/.pnpm/node_modules/package.json"),
)("@babel/parser");
const reports = process.argv.slice(2);
assert.equal(reports.length, 2, "Pass successful v1 and v2 sdk-delivery reports");
const versions = {};
for (const [index, relative] of reports.entries()) {
  const filename = fs.realpathSync(path.resolve(root, relative));
  assert(filename.startsWith(path.join(root, ".tmp/sdk-delivery") + path.sep));
  const bytes = fs.readFileSync(filename);
  const report = JSON.parse(bytes);
  assert.equal(report.status, "pass", "Use a fully verified generated fixture");
  const generation = report.generations.find((item) => item.count === 100);
  assert(generation, "Browser suite uses the representative 100-operation fixture");
  const sdkRoot = fs.realpathSync(path.resolve(root, generation.javascript));
  assert(sdkRoot.startsWith(path.dirname(filename) + path.sep));
  const assets = new Map();
  const visit = (directory) => {
    for (const item of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, item.name);
      assert(!item.isSymbolicLink());
      if (item.isDirectory()) visit(file);
      else if (item.name.endsWith(".js")) {
        const data = fs.readFileSync(file);
        assets.set(path.relative(sdkRoot, file), {
          data,
          sha256: hash(data),
          packed: brotliCompressSync(data, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }),
        });
      }
    }
  };
  visit(sdkRoot);
  for (const inventory of [
    generation.baseline.inventory,
    generation.bootstrap.inventory,
    ...report.workloads.filter((item) => item.count === 100).map((item) => item.selected.inventory),
  ]) {
    for (const item of inventory)
      assert.equal(assets.get(item.path)?.sha256, item.sha256, `Stale SDK asset ${item.path}`);
  }
  const entry = assets.get("selective/index.js").data.toString();
  const identity = entry.match(/generation: "([a-f0-9]{64})"/)[1];
  const edges = (name) => {
    const ast = parser.parse(assets.get(name).data.toString(), { sourceType: "module" });
    return ast.program.body
      .filter(
        (node) =>
          ["ImportDeclaration", "ExportNamedDeclaration", "ExportAllDeclaration"].includes(
            node.type,
          ) && node.source?.type === "StringLiteral",
      )
      .map((node) =>
        path.posix.normalize(path.posix.join(path.posix.dirname(name), node.source.value)),
      );
  };
  const graph = (starts) => {
    const pending = [...starts],
      seen = new Set();
    while (pending.length) {
      const name = pending.pop();
      if (seen.has(name)) continue;
      assert(assets.has(name), `Missing compiled SDK dependency ${name}`);
      seen.add(name);
      pending.push(...edges(name));
    }
    return [...seen].sort();
  };
  const lookup = (route) => {
    const digest = hash("route\0" + route);
    return "selective/lookup/r-" + digest.slice(0, 16) + "/" + digest.slice(16) + ".js";
  };
  const provider = (route) => edges(lookup(route))[0];
  const stages = [
    { name: "one", routes: ["GET /items/item0"] },
    { name: "post", routes: ["GET /items/item0", "POST /echo"] },
    { name: "xml", routes: ["GET /items/item0", "GET /xml"] },
    { name: "stream", routes: ["GET /items/item0", "GET /events"] },
    { name: "link-ready", routes: ["GET /source"] },
    { name: "link-invoked", routes: ["GET /source"], followLink: true },
  ].map((stage) => ({
    ...stage,
    selected: graph([
      "selective/index.js",
      "internal/runtime/client/selected-client.js",
      ...stage.routes.map(lookup),
      ...(stage.followLink ? [provider("GET /linked/{itemId}")] : []),
    ]),
  }));
  versions[index === 0 ? "v1" : "v2"] = {
    report: relative,
    reportSHA256: hash(bytes),
    identity,
    assets,
    graph,
    lookup,
    stages,
    baseline: graph(["index.js"]),
    bootstrap: graph(["selective/index.js"]),
  };
}
assert.notEqual(
  versions.v1.identity,
  versions.v2.identity,
  "A new URL alone does not prove cross-generation behavior",
);
const runID = randomUUID();
const directory = path.join(root, ".tmp/sdk-browser", runID);
fs.mkdirSync(directory, { recursive: true });
const log = path.join(directory, "requests.jsonl");
const cases = [];
for (let repetition = 0; repetition < 11; repetition++) {
  for (const kind of repetition % 2 === 0 ? ["baseline", "selected"] : ["selected", "baseline"]) {
    for (const phase of ["cold", "revisit", "new-generation", "retained-old"])
      cases.push({
        id: String(cases.length),
        repetition,
        kind,
        phase,
        version: phase === "new-generation" ? "v2" : "v1",
        variant: "normal",
      });
  }
}
for (const variant of [
  "missing",
  "mime",
  "mixed",
  "disconnect",
  "cross-origin",
  "cors-denied",
  "csp-denied",
  "preload",
])
  cases.push({
    id: String(cases.length),
    repetition: 0,
    kind: "selected",
    phase: variant,
    version: "v1",
    variant,
  });
const byID = new Map(cases.map((entry) => [entry.id, entry]));
const tlsCase = {
  id: "tls",
  repetition: 0,
  kind: "selected",
  phase: "https",
  version: "v1",
  variant: "tls",
};
byID.set(tlsCase.id, tlsCase);
let mainOrigin, assetOrigin, currentCase;
const enteredCases = new Map();
const prefixFor = (entry) =>
  `/sdk/${runID}/${entry.repetition}/${entry.variant}/${entry.version}/${entry.kind}/`;
const assetBase = (entry) =>
  (["cross-origin", "cors-denied", "csp-denied"].includes(entry.variant)
    ? assetOrigin
    : mainOrigin) + prefixFor(entry);
const negative = new Set(["missing", "mime", "mixed", "disconnect", "cors-denied", "csp-denied"]);
function page(entry, moduleOnly = false) {
  const version = versions[entry.version],
    prefix = prefixFor(entry),
    nonce = runID;
  const base = assetBase(entry);
  const preload =
    entry.variant === "preload"
      ? `<link rel="preload" as="script" href="${base}selective/all.js">`
      : "";
  const script = `
performance.setResourceTimingBufferSize(10000);
const entry=${JSON.stringify(entry)}, prefix=${JSON.stringify(prefix)}, base=(${JSON.stringify(["cross-origin", "cors-denied", "csp-denied"].includes(entry.variant) ? assetOrigin : "")}||location.origin)+prefix, stages=${JSON.stringify(version.stages)}, initial=${JSON.stringify(entry.kind === "baseline" ? version.baseline : version.bootstrap)};
const started=performance.now(), exercise=${exerciseGeneratedSDK.toString()};
const resources=()=>performance.getEntriesByType('resource').filter(row=>new URL(row.name).pathname.startsWith(prefix));
const snapshot=()=>resources().map(row=>({path:new URL(row.name).pathname.slice(prefix.length),body:row.encodedBodySize,transfer:row.transferSize}));
const expected=new Set(initial), results=[];let previous=new Set();
const record=(name, stageStarted)=>{const rows=snapshot(), paths=rows.map(row=>row.path);if(new Set(paths).size!==paths.length)throw Error('Duplicate module network entries');if(entry.variant!=='preload'&&JSON.stringify([...paths].sort())!==JSON.stringify([...expected].sort()))throw Error('Unexpected SDK graph at '+name+': '+JSON.stringify(paths));const added=rows.filter(row=>!previous.has(row.path));results.push({name,files:added.length,body:added.reduce((n,r)=>n+r.body,0),transfer:added.reduce((n,r)=>n+r.transfer,0),elapsedMS:performance.now()-stageStarted});previous=new Set(paths);};
try {
 const bad=${JSON.stringify(negative.has(entry.variant))};
 if(bad){
  let error,apiCalls=0;
  try{const sdk=await import(base+'selective/index.js');const prepared=await sdk.loadOperations([sdk.routes['GET /items/item0']]);sdk.createClient({operations:prepared,fetch:async()=>{apiCalls++;return Response.json({});}});}
  catch(value){error=value;}
  if(!error)throw Error('Expected module failure did not occur');
  const expectedStage=entry.variant==='mixed'?'IDENTITY':(['cors-denied','csp-denied'].includes(entry.variant)?undefined:'MODULE_LOAD');
  if(expectedStage&&error.stage!==expectedStage)throw Error('Wrong failure stage '+error.stage);
  if(apiCalls!==0)throw Error('Module failure sent an API request');
  const result={...entry,pass:true,expectedFailure:true,stage:error.stage??null,apiCalls,elapsedMS:performance.now()-started};
  console.log('SDK_BROWSER_CASE '+JSON.stringify(result));document.querySelector('#result').textContent=JSON.stringify(result);parent.postMessage({type:'sdk-result',result},location.origin);
 }else{
  if(!isSecureContext||!crypto.subtle)throw Error('Namespace lookup requires a secure context');
  const module=await import(base+(entry.kind==='baseline'?'index.js':'selective/index.js'));
  record('initial',started);
  for(const step of stages){const time=performance.now();await exercise(module,step.routes,{selected:entry.kind==='selected',followLink:step.followLink===true});if(entry.kind==='selected')for(const name of step.selected)expected.add(name);record(step.name,time);}
  if(entry.variant==='preload'){await new Promise(resolve=>setTimeout(resolve,100));const rows=snapshot();if(!rows.some(row=>row.path==='selective/all.js'))throw Error('Preloaded names cost was not observed');}
  const final=snapshot();
  const result={...entry,pass:true,stages:results,elapsedMS:performance.now()-started,paths:final.map(row=>row.path).sort(),body:final.reduce((n,r)=>n+r.body,0),transfer:final.reduce((n,r)=>n+r.transfer,0),secureContext:isSecureContext,protocol:location.protocol,origin:location.origin};
  console.log('SDK_BROWSER_CASE '+JSON.stringify(result));
  if(entry.variant==='tls')console.log('SDK_BROWSER_HTTPS '+JSON.stringify({id:entry.id,pass:true,protocol:location.protocol,origin:location.origin,secureContext:isSecureContext,files:final.length,stages:results.length}));
  document.querySelector('#result').textContent=JSON.stringify(result);parent.postMessage({type:'sdk-result',result},location.origin);
 }
}catch(error){const result={...entry,pass:false,error:String(error?.stack??error)};console.error('SDK_BROWSER_CASE '+JSON.stringify(result));document.querySelector('#result').textContent=JSON.stringify(result);parent.postMessage({type:'sdk-result',result},location.origin);}
`;
  if (moduleOnly) return script;
  // The host-allowlist control starts from a same-origin module without a nonce.
  const bootstrap =
    entry.variant === "csp-denied"
      ? `<script type="module" src="/bootstrap/${entry.id}.js"></script>`
      : `<script type="module" nonce="${nonce}">${script}</script>`;
  return `<!doctype html><meta charset="utf-8"><title>Generated SDK case</title>${preload}<pre id="result">Running</pre>${bootstrap}`;
}
const suite = `<!doctype html><meta charset="utf-8"><title>Generated SDK delivery</title><h1>Generated SDK delivery</h1><pre id="summary">Starting</pre><iframe id="case"></iframe><script type="module" nonce="${runID}">
const cases=${JSON.stringify(cases)},results=[],frame=document.querySelector('#case');let index=0;
window.addEventListener('message',event=>{if(event.origin!==location.origin||event.source!==frame.contentWindow||event.data?.type!=='sdk-result')return;const result=event.data.result;if(result.id!==cases[index]?.id)return;results.push(result);index++;document.querySelector('#summary').textContent=JSON.stringify({completed:index,total:cases.length,failed:results.filter(row=>!row.pass).length});if(index<cases.length)frame.src='/case/'+cases[index].id;else{const groups=new Map();for(const row of results){const key=row.kind+':'+row.phase;let group=groups.get(key);if(!group){group={kind:row.kind,phase:row.phase,passed:0,signatures:[],elapsedMS:[],errors:[]};groups.set(key,group);}if(row.pass){group.passed++;group.elapsedMS.push(row.elapsedMS);group.signatures.push(row.expectedFailure?'expected:'+row.stage:row.stages.map(stage=>[stage.files,stage.body,stage.transfer].join('/')).join(','));}else group.errors.push(row.error);}for(const group of groups.values())console.log('SDK_BROWSER_GROUP '+JSON.stringify(group));console.log('SDK_BROWSER_SUMMARY '+JSON.stringify({runID:${JSON.stringify(runID)},cases:results.length,passed:results.filter(row=>row.pass).length}));}});frame.src='/case/'+cases[0].id;
</script>`;
const serve = (request, response) => {
  const url = new URL(request.url ?? "/", mainOrigin ?? "http://localhost");
  let data,
    type,
    asset,
    category = "harness",
    status = 200;
  let requestCase = currentCase;
  const extra = {};
  if (request.method === "GET" && url.pathname === "/") {
    data = Buffer.from(suite);
    type = "text/html; charset=utf-8";
  } else if (request.method === "GET" && url.pathname.startsWith("/case/")) {
    const entry = byID.get(url.pathname.slice(6));
    if (entry) {
      currentCase = entry;
      requestCase = entry;
      enteredCases.set(prefixFor(entry), entry);
      data = Buffer.from(page(entry));
      type = "text/html; charset=utf-8";
      extra["content-security-policy"] =
        entry.variant === "csp-denied"
          ? "default-src 'none'; script-src 'self'; frame-src 'self'; connect-src 'none'"
          : `default-src 'none'; script-src 'nonce-${runID}' 'self' ${assetOrigin}; frame-src 'self'; connect-src 'none'`;
    }
  } else if (request.method === "GET" && url.pathname.startsWith("/bootstrap/")) {
    const entry = byID.get(url.pathname.slice(11).replace(/\.js$/, ""));
    if (entry?.variant === "csp-denied") {
      requestCase = entry;
      data = Buffer.from(page(entry, true));
      type = "text/javascript; charset=utf-8";
    }
  } else if (request.method === "GET" && url.pathname.startsWith("/sdk/")) {
    const entry = [...enteredCases.values()].find((value) =>
      url.pathname.startsWith(prefixFor(value)),
    );
    if (!entry) {
      response.writeHead(404);
      response.end();
      return;
    }
    requestCase = entry;
    asset = url.pathname.slice(prefixFor(entry).length);
    category = "sdk";
    let version = versions[entry.version];
    const lookup = version.lookup("GET /items/item0");
    if (entry.variant === "disconnect" && asset === lookup) {
      fs.appendFileSync(
        log,
        JSON.stringify({
          caseID: entry.id,
          category,
          method: request.method,
          path: url.pathname,
          asset,
          status: 0,
          bodyBytes: 0,
          disconnected: true,
        }) + "\n",
      );
      request.socket.destroy();
      return;
    }
    if (
      entry.variant === "mixed" &&
      (asset.startsWith("selective/lookup/") || asset.startsWith("internal/executions/"))
    )
      version = versions.v2;
    const value = version.assets.get(asset);
    if (value && !(entry.variant === "missing" && asset === lookup)) {
      data = value.packed;
      type =
        entry.variant === "mime" && asset === lookup
          ? "text/plain"
          : "text/javascript; charset=utf-8";
      extra["content-encoding"] = "br";
    }
    if (entry.variant !== "cors-denied") {
      extra["access-control-allow-origin"] = mainOrigin;
      extra["timing-allow-origin"] = mainOrigin;
    }
  }
  if (data === undefined) {
    status = 404;
    data = Buffer.from("Not found");
    type = "text/plain";
  }
  response.writeHead(status, {
    "content-type": type,
    "content-length": data.length,
    "x-content-type-options": "nosniff",
    "cache-control":
      category === "sdk" && status === 200 ? "public, max-age=31536000, immutable" : "no-store",
    ...extra,
  });
  response.end(data);
  fs.appendFileSync(
    log,
    JSON.stringify({
      caseID: requestCase?.id,
      category,
      method: request.method,
      path: url.pathname,
      asset,
      status,
      bodyBytes: data.length,
    }) + "\n",
  );
};
const assetServer = http.createServer(serve),
  server = http.createServer(serve);
await new Promise((resolve) => assetServer.listen(0, "127.0.0.1", resolve));
assetOrigin = `http://127.0.0.1:${assetServer.address().port}`;
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
mainOrigin = `http://127.0.0.1:${server.address().port}`;
const manifest = {
  runID,
  origin: mainOrigin,
  assetOrigin,
  cases,
  tlsCase,
  scope:
    "Actual generated SDK over loopback HTTP in a browser secure context; injected browser fetch for API semantics. Real distinct generated versions, native ESM, cross-origin/CSP/MIME/failure/cache. Not an external CDN latency or TLS deployment benchmark.",
  versions: Object.fromEntries(
    Object.entries(versions).map(([name, value]) => [
      name,
      {
        report: value.report,
        reportSHA256: value.reportSHA256,
        identity: value.identity,
        baseline: value.baseline,
        bootstrap: value.bootstrap,
        stages: value.stages,
        assets: Object.fromEntries(
          [...value.assets].map(([file, item]) => [
            file,
            { sha256: item.sha256, raw: item.data.length, brotli5: item.packed.length },
          ]),
        ),
      },
    ]),
  ),
};
fs.writeFileSync(path.join(directory, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
console.log(
  JSON.stringify({
    ready: true,
    origin: mainOrigin,
    assetOrigin,
    runID,
    cases: cases.length,
    directory: path.relative(root, directory),
  }),
);
const stop = () => {
  server.close(() => assetServer.close(() => process.exit(0)));
};
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
