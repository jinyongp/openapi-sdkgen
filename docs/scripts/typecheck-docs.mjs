import { createRequire } from "node:module";
import { readFileSync, realpathSync, readdirSync } from "node:fs";
import { resolve, dirname } from "node:path";
const root = resolve(import.meta.dirname, "../..");
const require = createRequire(realpathSync(resolve(root,"docs/node_modules/vitepress/package.json")));
const vueRequire = createRequire(require.resolve("vue/package.json"));
const { parse } = vueRequire("vue/compiler-sfc");
// The existing TS 6 alias retains the JavaScript compiler API; TS 7 uses a native CLI.
const ts = (await import(resolve(root,"test/typescript/node_modules/typescript-6/lib/typescript.js"))).default;
const virtual = new Map();
function scan(directory) {
 const files=[];
 for(const entry of readdirSync(directory,{withFileTypes:true})) {
  const path=resolve(directory,entry.name);
  if(entry.isDirectory() && !["dist","cache"].includes(entry.name)) files.push(...scan(path));
  else if(/\.(?:ts|mts)$/.test(path)) files.push(path);
  else if(path.endsWith('.vue')) {
   const source=readFileSync(path,'utf8'),{descriptor}=parse(source,{filename:path});
   if(descriptor.scriptSetup?.lang!=='ts')continue;
   const file=path+'.ts';virtual.set(file,`${descriptor.script?.content ?? ""}\n${descriptor.scriptSetup.content}\nexport default {} as import("vue").DefineComponent;`);files.push(file);
  }
 }
 return files;
}
const files=scan(resolve(root,'docs/.vitepress')).filter(path=>!path.includes('/dist/')&&!path.includes('/cache/'));
const shim=resolve(root,'docs/.tmp/docs-type-shim.d.ts');
virtual.set(shim,'declare const defineProps: typeof import("vue").defineProps; declare const withDefaults: typeof import("vue").withDefaults; declare const defineEmits: typeof import("vue").defineEmits; declare const defineExpose: typeof import("vue").defineExpose; declare module "*.vue" { import type { DefineComponent } from "vue"; const component: DefineComponent; export default component; }');files.push(shim);
files.push(resolve(dirname(require.resolve('vite/package.json')), 'client.d.ts'));
const options={noEmit:true,strict:true,skipLibCheck:true,allowJs:true,allowImportingTsExtensions:true,module:ts.ModuleKind.ESNext,moduleResolution:ts.ModuleResolutionKind.Bundler,target:ts.ScriptTarget.ES2022,lib:['lib.es2022.d.ts','lib.dom.d.ts','lib.dom.iterable.d.ts'],paths:{vue:[dirname(require.resolve('vue/package.json'))]}};
const host=ts.createCompilerHost(options),read=host.readFile.bind(host),exists=host.fileExists.bind(host);
host.readFile=path=>virtual.get(path)??read(path);host.fileExists=path=>virtual.has(path)||exists(path);
const program=ts.createProgram(files,options,host);
const diagnostics=ts.getPreEmitDiagnostics(program);
if(diagnostics.length){console.error(ts.formatDiagnosticsWithColorAndContext(diagnostics,{getCanonicalFileName:path=>path,getCurrentDirectory:()=>root,getNewLine:()=>"\n"}));process.exitCode=1;}
else console.log(`ok documentation TypeScript: ${files.length} modules including Vue setup scripts`);
