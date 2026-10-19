import assert from "node:assert/strict";
import { test } from "node:test";
import { cp, mkdtemp, rm, writeFile, appendFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { validateLocaleStructure } from "./validate-locales.mjs";
const root = resolve(import.meta.dirname, "..");
async function fixture(mutate, expected) {
 const directory=await mkdtemp(resolve(tmpdir(),"sdk-docs-"));
 try {
  for (const path of ["typescript","golang","ko","guide","examples","reference","index.md","playground.md","public"]) await cp(resolve(root,path),resolve(directory,path),{recursive:true});
  if (mutate) await mutate(directory);
  if (expected) await assert.rejects(validateLocaleStructure({docsRoot:directory}),expected);
  else await validateLocaleStructure({docsRoot:directory});
 } finally {await rm(directory,{recursive:true,force:true});}
}
test("all SDK scopes and historical routes are present",()=>fixture());
test("missing Korean SDK counterpart fails",()=>fixture(dir=>rm(resolve(dir,"ko/golang/guide/client.md")),/client/));
test("undeclared SDK page fails",()=>fixture(dir=>writeFile(resolve(dir,"typescript/guide/extra.md"),"# Extra"),/Undeclared/));
test("broken SDK link fails",()=>fixture(dir=>appendFile(resolve(dir,"golang/guide/client.md"),"\n[Broken](./missing.md)\n"),/missing link/));
test("missing public download fails",()=>fixture(dir=>rm(resolve(dir,"public/examples/golang-todo.json")),/missing link/));
test("legacy pages must opt out of search",()=>fixture(dir=>writeFile(resolve(dir,"guide/client.md"),"# Old page"),/Invalid historical/));
