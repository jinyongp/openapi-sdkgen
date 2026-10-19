/** Supplement static assertions with fixed-u constructor traces of pinned Test262.
 * Executes only the fixed test/harness source in disposable VM contexts. This is
 * fixture preparation, never an SDK runtime or a full Test262 conformance claim.
 */
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
import vm from 'node:vm';
import { gzipSync } from 'node:zlib';
if(process.versions.unicode!=='17.0')throw Error('Preparation requires a Unicode17 oracle');
const root=resolve(import.meta.dirname,'../..');
const inventory=JSON.parse(readFileSync(resolve(root,'testdata/goruntime/corpora.json'),'utf8'));
const pinned=new Map(inventory.files.filter(f=>f.suite==='test262').map(f=>[f.path,f]));
function source(path) {
 const item=pinned.get(path);if(!item)throw Error(`Unpinned source ${path}`);
 const data=readFileSync(resolve(root,'.tmp/goruntime-corpora/test262',path));
 if(createHash('sha256').update(data).digest('hex')!==item.sha256)throw Error(`Source hash ${path}`);
 return data.toString('utf8');
}
const cases=new Map(),files=[];
const instrument=`(() => {
 const nativeExec=RegExp.prototype.exec, apply=Reflect.apply;
 const descriptor=Object.getOwnPropertyDescriptor;
 const source=descriptor(RegExp.prototype,'source').get;
 const flags=Object.fromEntries(['unicode','unicodeSets','ignoreCase','multiline','dotAll','sticky'].map(f=>[f,descriptor(RegExp.prototype,f).get]));
 const record=__record;delete globalThis.__record;
 RegExp.prototype.exec=function exec(input){
  let pattern, modifiers='', eligible=false, reason='object-or-other-flags';
  try {
   if(typeof input==='string'&&apply(flags.unicode,this,[])&&!apply(flags.unicodeSets,this,[])&&!apply(flags.sticky,this,[])&&this.lastIndex===0){
    pattern=apply(source,this,[]);for(const [key,flag]of [['ignoreCase','i'],['multiline','m'],['dotAll','s']])if(apply(flags[key],this,[]))modifiers+=flag;
    eligible=true;
   }
  }catch{}
  const result=apply(nativeExec,this,[input]);
  if(eligible)record(modifiers?'(?'+modifiers+':'+pattern+')':pattern,input,result!==null,'fixed-u');
  else record('','',false,reason);
  return result;
 };
})();`;
function units(value){return Array.from({length:value.length},(_,i)=>value.charCodeAt(i));}
for(const entry of pinned.values()){
 if(!entry.path.startsWith('test/'))continue;
 const text=source(entry.path),meta=text.match(/\/\*---([\s\S]*?)---\*\//)?.[1]??'';
 const row={path:entry.path,sourceSHA256:entry.sha256,traces:0,otherFlagOrObjectCalls:0,oversizedInputs:0,status:''};
 if(/negative:\s*\n\s*phase:\s*(parse|early)/.test(meta)){
  row.status='negative syntax fixture: static compiler adapter; upstream JS not executed';files.push(row);continue;
 }
 const pending=new Map();
 const context=vm.createContext({__record(pattern,input,matches,kind){
  if(kind!=='fixed-u'){row.otherFlagOrObjectCalls++;return;}
  if(input.length>8192||pattern.length>32768){row.oversizedInputs++;return;}
  row.traces++;
  const key=JSON.stringify([pattern,input]);
  if(pending.size>=10000&&!pending.has(key))throw Error('fixture trace limit');
  pending.set(key,{patternUnits:units(pattern),inputUnits:units(input),matches,file:entry.path});
 }});
 try{
  for(const harness of ['sta.js','assert.js',...(meta.match(/includes:\s*\[([^\]]*)\]/)?.[1]??'').split(',').map(s=>s.trim()).filter(Boolean)]){
   vm.runInContext(source('harness/'+harness),context,{timeout:250,filename:harness});
  }
  vm.runInContext(instrument,context,{timeout:250});
  vm.runInContext((/flags:\s*\[[^\]]*onlyStrict/.test(meta)?'"use strict";\n':'')+text,context,{timeout:250,filename:entry.path});
  for(const [key,c]of pending){const old=cases.get(key);if(old&&old.matches!==c.matches)throw Error('state-dependent fixed-u trace');}
  for(const [key,c]of pending)if(!cases.has(key))cases.set(key,c);
  row.status=row.traces?'supplementary fixed-u traces captured; instrumented upstream assertions passed':row.oversizedInputs?'fixed-u invocations exceed preparation size limit; original source and Unicode boundary contracts retained':'no fixed-u string matching invocation; API or other-mode source retained';
 }catch(error){row.status='supplementary JS execution incomplete; static adapter and independent contracts remain authoritative';row.reason=String(error).slice(0,300);}
 files.push(row);
}
const value={source:inventory.sources.test262,oracle:{node:process.version,unicode:process.versions.unicode},scope:'Supplementary Node Boolean traces only. Only u-mode strings, non-sticky, lastIndex0; global state/JS object APIs/other modes excluded. Scoped i/m/s modeled. Limits are preparation coverage limits, not SDK limits. Unexecuted or oversized traces remain explicit; no full Test262 pass.',cases:[...cases.values()]};
const json=JSON.stringify(value)+'\n';
writeFileSync(resolve(root,'testdata/goruntime/regexp-traces.json.gz'),gzipSync(json,{level:9}));
writeFileSync(resolve(root,'testdata/goruntime/regexp-trace-coverage.json'),JSON.stringify({source:value.source,oracle:value.oracle,scope:value.scope,files},null,2)+'\n');
console.log(JSON.stringify({cases:cases.size,files:files.length,capturedFiles:files.filter(f=>f.traces&&f.status.startsWith('supplementary fixed')).length,incompleteFiles:files.filter(f=>f.reason).length,oversizedInputs:files.reduce((n,f)=>n+f.oversizedInputs,0)}));
