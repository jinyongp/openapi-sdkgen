import assert from "node:assert/strict";
import { test } from "node:test";
import { legacyRoutes } from "../.vitepress/legacy-routes.mjs";
import { legacyDestination } from "../.vitepress/legacy-destination.ts";
test("all historical headings retain destination meaning",()=>{
 for(const entry of legacyRoutes) for(const anchor of entry.anchors) assert.equal(legacyDestination(entry.route,`https://docs.invalid${entry.route}.html#${encodeURIComponent(anchor)}`),entry.fragments[anchor]??`${entry.destination}#${anchor}`);
});
test("historical Playground target query beats preferences and filters unsafe query",()=>{
 assert.equal(legacyDestination("/playground","/playground?target=golang&example=todo-api&with=&url=private"),"/golang/playground?example=todo-api&with=");
 assert.equal(legacyDestination("/ko/playground","/ko/playground?target=unknown"),"/ko/typescript/playground");
});
test("unknown and malformed anchors drop instead of inventing a section",()=>{
 assert.equal(legacyDestination("/guide/client","/guide/client#missing"),"/typescript/guide/client");
 assert.equal(legacyDestination("/guide/client","/guide/client#%zz"),"/typescript/guide/client");
});
