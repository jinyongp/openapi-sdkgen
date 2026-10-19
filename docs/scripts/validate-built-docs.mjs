import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { topics, sdkTargets, displayLocales, sdkPath, docsBase, publishedTargets } from "../.vitepress/docs-catalog.mjs";
import { legacyRoutes } from "../.vitepress/legacy-routes.mjs";
import { publicURL } from "../.vitepress/metadata.mjs";
const dist=resolve(import.meta.dirname,"../.vitepress/dist");
const fileFor=route=>resolve(dist,`.${route}${route.endsWith('/')?'index':''}.html`);
const contents=new Map();
const html=async route=>{if(!contents.has(route))contents.set(route,await readFile(fileFor(route),'utf8'));return contents.get(route);};
assert.deepEqual(publishedTargets,["typescript","golang"],"Both SDKs must ship");
for(const sdk of sdkTargets)for(const locale of displayLocales)for(const topic of topics){
 const route=sdkPath(sdk,locale,topic.id),source=await html(route);
 assert.ok(source.includes(`<html lang="${locale==='ko'?'ko-KR':'en-US'}"`),route);
 assert.ok(source.includes(`rel="canonical" href="${publicURL(route)}"`),route);
 for(const language of displayLocales)assert.ok(source.includes(`hreflang="${language}" href="${publicURL(sdkPath(sdk,language,topic.id))}"`),route);
}
let anchors=0;
for(const entry of legacyRoutes){
 const source=await html(entry.route);
 assert.ok(source.includes('content="noindex,follow"'),entry.route);
 assert.ok(source.includes(`rel="canonical" href="${publicURL(entry.destination)}"`),entry.route);
 assert.ok(source.includes(`href="${docsBase}${entry.destination.slice(1)}`),`No-JS link: ${entry.route}`);
 for(const anchor of entry.anchors){
  const destination=entry.fragments[anchor]??`${entry.destination}#${anchor}`;
  const [route,fragment]=destination.split('#');
  assert.ok((await html(route)).includes(`id="${fragment}"`),`${entry.route}#${anchor} => ${destination}`);anchors++;
 }
}
const modules=[];
for(const file of await readdir(resolve(dist,'assets'),{recursive:true})){
 if(!file.endsWith('.js'))continue;
 const path=resolve(dist,'assets',file),source=await readFile(path,'utf8');
 if(!source.includes('documentIds')||!source.includes('serializationVersion')||source.includes('document.createElement'))continue;
 let parsed;try{const exported=(await import(pathToFileURL(path).href)).default;if(typeof exported==='string')parsed=JSON.parse(exported);}catch{continue;}
 if(parsed?.documentIds)modules.push(Object.values(parsed.documentIds));
}
const counts={};
for(const sdk of sdkTargets)for(const locale of displayLocales){
 const prefix=`${docsBase}${locale==='ko'?'ko/':''}${sdk}/`;
 const index=modules.find(ids=>ids.some(id=>id.startsWith(prefix)));
 assert.ok(index,`Search index missing: ${prefix}`);
 assert.ok(index.every(id=>id.startsWith(prefix)),`Search scope leak: ${prefix}`);
 counts[`${locale}/${sdk}`]=new Set(index.map(id=>id.split('#')[0])).size;
}
for(const ids of modules)for(const id of ids){const route=id.split('#')[0].replace(docsBase,'/').replace(/\.html$/,'');assert.ok(!legacyRoutes.some(entry=>entry.route===route),`Legacy indexed: ${id}`);}
console.log(JSON.stringify({pages:114,legacy:legacyRoutes.length,anchors,searchPages:counts}));
