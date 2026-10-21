// Diagnostic for two real Graph providers whose large output closures share a
// block. Bundles include their actual runtime imports; no source rewriting.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {createRequire} from "node:module";
import {spawnSync} from "node:child_process";
import {pathToFileURL} from "node:url";
import {gzipSync} from "node:zlib";
const [baselineArg,candidateArg,outputArg]=process.argv.slice(2);
assert(baselineArg && candidateArg && outputArg,"BASELINE_SDK CANDIDATE_SDK FRESH_OUTPUT required");
const roots={baseline:path.resolve(baselineArg),candidate:path.resolve(candidateArg)};
const output=path.resolve(outputArg);
fs.mkdirSync(output,{mode:0o700});
function* walk(directory){
  for(const entry of fs.readdirSync(directory,{withFileTypes:true}).sort((a,b)=>Buffer.compare(Buffer.from(a.name),Buffer.from(b.name)))){
    const file=path.join(directory,entry.name);
    if(entry.isDirectory())yield* walk(file);else if(file.endsWith('.ts'))yield file;
  }
}
const chosen=[],bundles=new Set();
for(const file of walk(path.join(roots.candidate,'internal/executions'))){
  const source=fs.readFileSync(file,'utf8');
  const match=/import \{ schemas as outputSchemas \} from ("(?:[^"\\]|\\.)*")/.exec(source);
  if(!match)continue;
  const target=path.resolve(path.dirname(file),JSON.parse(match[1])).replace(/\.js$/,'.ts');
  if(bundles.has(target)||!fs.readFileSync(target,'utf8').includes('mergeSchemas('))continue;
  const route=/\broute: ("(?:[^"\\]|\\.)*")/.exec(source);
  assert(route,'provider route required');
  bundles.add(target);chosen.push({file:path.relative(roots.candidate,file),route:JSON.parse(route[1])});
  if(chosen.length===2)break;
}
assert.equal(chosen.length,2,'two distinct real shared closures required');
const require=createRequire(path.join(process.cwd(),'test/typescript/package.json'));
const viteRequire=createRequire(fs.realpathSync(require.resolve('vite/package.json')));
const {rolldown}=await import(pathToFileURL(viteRequire.resolve('rolldown')));
const sizes={};
for(const [variant,root] of Object.entries(roots)){
  const directory=path.join(output,variant);fs.mkdirSync(directory);
  const entry=path.join(directory,'entry.ts');
  fs.writeFileSync(entry,chosen.map((item,index)=>`import {provider as p${index}} from ${JSON.stringify(path.join(root,item.file))};`).join('\n')+'\nexport const providers=[p0,p1];\n');
  const warnings=[];
  const build=await rolldown({input:entry,platform:'node',treeshake:true,onLog:(_level,log)=>warnings.push(log.message)});
  try{
    const {output:chunks}=await build.generate({format:'esm',minify:true,entryFileNames:'entry.mjs',chunkFileNames:'chunk-[hash].mjs'});
    assert.equal(warnings.length,0,warnings.join('\n'));
    const javascript=chunks.filter(item=>item.type==='chunk');
    sizes[variant]={jsBytes:javascript.reduce((n,item)=>n+Buffer.byteLength(item.code),0),gzipBytes:javascript.reduce((n,item)=>n+gzipSync(item.code,{level:6}).length,0)};
    for(const chunk of javascript)fs.writeFileSync(path.join(directory,chunk.fileName),chunk.code);
  }finally{await build.close();}
}
const script=`const start=performance.now();const {providers}=await import(process.argv[1]);const elapsedMilliseconds=performance.now()-start;global.gc();console.log(JSON.stringify({elapsedMilliseconds,retainedHeapBytes:process.memoryUsage().heapUsed,routes:providers.map(provider=>provider.route)}));`;
const expected=chosen.map(item=>item.route);
const rows=[];
for(let pair=-1;pair<10;pair++){
  const order=pair%2===0?['baseline','candidate']:['candidate','baseline'];
  for(const variant of order){
    const result=spawnSync(process.execPath,['--expose-gc','--input-type=module','--eval',script,pathToFileURL(path.join(output,variant,'entry.mjs')).href],{encoding:'utf8',timeout:60000,maxBuffer:1024*1024});
    assert.equal(result.status,0,`${result.error??result.stderr} signal=${result.signal}`);
    const row={pair,variant,...JSON.parse(result.stdout)};assert.deepEqual(row.routes,expected);
    rows.push(row);fs.writeFileSync(path.join(output,'trials.json'),JSON.stringify(rows,null,2)+'\n');
  }
}
const ratios=Array.from({length:10},(_,pair)=>rows.find(row=>row.pair===pair&&row.variant==='candidate').elapsedMilliseconds/rows.find(row=>row.pair===pair&&row.variant==='baseline').elapsedMilliseconds);
let seed=21749;
const boot=[];
for(let i=0;i<5000;i++){
  let sum=0;
  for(let j=0;j<ratios.length;j++){seed=(Math.imul(seed,1664525)+1013904223)>>>0;sum+=Math.log(ratios[seed%ratios.length]);}
  boot.push(Math.exp(sum/ratios.length));
}
boot.sort((a,b)=>a-b);
const summary={chosen,sizes,node:process.version,bundler:viteRequire('rolldown/package.json').version,pairs:10,ratios,geometricMeanRatio:Math.exp(ratios.reduce((sum,value)=>sum+Math.log(value),0)/ratios.length),bootstrap95:[boot[125],boot[4874]],boundary:'fresh Node processes; module import and initialization only; filesystem cache warm/unspecified; first pair excluded warmup; explicit GC after timing for retained heap',status:boot[4874]<=1.05?'pass':'inconclusive'};
fs.writeFileSync(path.join(output,'summary.json'),JSON.stringify(summary,null,2)+'\n');
console.log(JSON.stringify(summary));
if(summary.status!=='pass')process.exitCode=1;
