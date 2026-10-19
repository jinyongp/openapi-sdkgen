import assert from "node:assert/strict";
import { test } from "node:test";
import { readSDKOptions, writeSDKOptions } from "../.vitepress/theme/playground/options.ts";
import { readPlaygroundPreferences, writePlaygroundPreferences } from "../.vitepress/theme/playground/preferences.ts";
const memory = () => { const values = new Map(); return {getItem: key => values.get(key) ?? null, setItem: (key,value) => values.set(key,value)}; };
test("options have per-SDK storage with explicit URL precedence including empty addons", () => {
 const store=memory(); writeSDKOptions(store,"golang",{goImportPath:"example.test/api",goPackage:"sdk",server:true,metadata:true});
 assert.equal(readSDKOptions(store,"typescript",new URLSearchParams()).server,false);
 const actual=readSDKOptions(store,"golang",new URLSearchParams("with=&go-package=other"));
 assert.equal(actual.server,false); assert.equal(actual.metadata,false);assert.equal(actual.goPackage,"other");assert.equal(actual.goImportPath,"example.test/api");
});
test("corrupt and blocked storage use safe defaults", () => {
 const blocked={getItem(){throw Error("blocked")},setItem(){throw Error("blocked")}};
 assert.equal(readSDKOptions(blocked,"golang",new URLSearchParams()).goPackage,"api");
 writeSDKOptions(blocked,"typescript",{goImportPath:"x",goPackage:"api",server:false,metadata:false});
 const corrupt=memory();corrupt.setItem("openapi-sdkgen.playground.options.golang.v1",'{"server":"yes","goPackage":2}');
 assert.equal(readSDKOptions(corrupt,"golang",new URLSearchParams()).goPackage,"api");assert.equal(readSDKOptions(corrupt,"golang",new URLSearchParams()).server,false);
});
test("tree expansion is SDK-specific and the theme keeps its existing key", () => {
 const store=memory();writePlaygroundPreferences(store,{codeTheme:"dark",expandedPaths:new Set(["models"])},"golang");
 assert.equal(readPlaygroundPreferences(store,["dark"],"typescript").codeTheme,"dark");
 assert.deepEqual(readPlaygroundPreferences(store,["dark"],"typescript").expandedPaths,[]);
 assert.deepEqual(readPlaygroundPreferences(store,["dark"],"golang").expandedPaths,["models"]);
});
