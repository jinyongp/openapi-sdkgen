import { mkdir, writeFile, readFile, readdir } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import assert from 'node:assert/strict';
const projectRoot=resolve(process.argv[2] ?? process.cwd());
const { build }=await import(pathToFileURL(join(projectRoot,'docs/node_modules/vitepress/dist/node/index.js')).href);
const { getLocaleForPath }=await import(pathToFileURL(join(projectRoot,'docs/node_modules/vitepress/dist/client/shared.js')).href);
const { publicDocRoutes }=await import(pathToFileURL(join(projectRoot,'docs/.vitepress/navigation.mjs')).href);
const root=join(projectRoot,'docs/.tmp/sdk-selection-plan-probe');
const scopes=['ko/typescript','ko/golang','typescript','golang','ko','root'];
const locales=Object.fromEntries(scopes.map(s=>[s,{label:'',lang:s.startsWith('ko')?'ko-KR':'en-US'}]));
for(const s of scopes){ const path=s==='root'?'index.md':`${s}/index.md`;assert.equal(getLocaleForPath({locales},path),s); }
assert.equal(getLocaleForPath({locales:{ko:{},'ko/golang':{}}},'ko/golang/index.md'),'ko');
await mkdir(join(root,'.vitepress'),{recursive:true});
await writeFile(join(root,'.vitepress/config.mjs'),`export default ${JSON.stringify({base:'/openapi-sdkgen/',locales,themeConfig:{search:{provider:'local'}}})}`);
for(const s of scopes){ const dir=s==='root'?root:join(root,s);await mkdir(dir,{recursive:true});await writeFile(join(dir,'index.md'),`# ${s} scope\n\nMARKER_${s.replaceAll('/','_')}\n`);}
await build(root);
const assets=join(root,'.vitepress/dist/assets');const matches=(await readdir(assets,{recursive:true})).filter(s=>s.endsWith('.js'));
const modules=[];
for(const name of matches){const text=await readFile(join(assets,name),'utf8');if(text.length>10000 || !text.includes('documentIds') || !text.includes('serializationVersion'))continue;const data=JSON.parse((await import(pathToFileURL(join(assets,name)).href)).default);modules.push({name,documentIds:Object.values(data.documentIds)});}
for(const s of scopes){const expected=`/openapi-sdkgen/${s==='root'?'':s+'/'}`;const found=modules.filter(m=>m.documentIds.some(id=>id.split('#')[0]===expected));assert.equal(found.length,1,s);assert.equal(found[0].documentIds.length,1,s);}
for(const s of scopes){const file=join(root,'.vitepress/dist',s==='root'?'':s,'index.html');const text=await readFile(file,'utf8');assert.ok(text.includes(`<html lang="${s.startsWith('ko')?'ko-KR':'en-US'}"`));}
const result={version:JSON.parse(await readFile(join(projectRoot,'docs/node_modules/vitepress/package.json'),'utf8')).version,publicRoutes:publicDocRoutes,scopeCount:scopes.length,nestedLocalePriorityVerified:true,wrongOrderReproduced:true,staticBuildPassed:true,htmlLanguageVerified:true,searchDocumentIDsIsolated:true,modules};
await writeFile(join(root,'result.json'),JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result));
