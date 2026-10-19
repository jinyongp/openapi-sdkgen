import assert from "node:assert/strict";
import { test } from "node:test";
import { topics, sdkPath, documentationScopes } from "../.vitepress/docs-catalog.mjs";
import { legacyRoutes } from "../.vitepress/legacy-routes.mjs";
import { parseDocContext, contextDestination, withDocsBase, readSDKPreference } from "../.vitepress/doc-context.ts";

test("all SDK/locale/topic paths round trip under the deployment base", () => {
  for (const sdk of ["typescript", "golang"]) for (const locale of ["en", "ko"]) for (const topic of topics) {
    assert.deepEqual(parseDocContext(withDocsBase(sdkPath(sdk, locale, topic.id))), { sdk, locale, topic: topic.id });
  }
  assert.deepEqual(parseDocContext("/openapi-sdkgen/ko/golang/guide/client.html?target=typescript#call"), {sdk:"golang",locale:"ko",topic:"client"});
  assert.equal(withDocsBase(withDocsBase("/golang/guide/client")), "/openapi-sdkgen/golang/guide/client");
  assert.equal(parseDocContext("/golangish/guide/client"), null);
  assert.equal(parseDocContext("/ko/golang/unknown"), null);
});
test("switches retain topic and locale; missing topic has explicit fallback", () => {
  assert.deepEqual(contextDestination({ sdk:"typescript",locale:"ko",topic:"client" },"golang","ko"), {path:"/ko/golang/guide/client",fallback:false});
  assert.deepEqual(contextDestination({ sdk:"golang",locale:"ko",topic:"missing" },"typescript","en"), {path:"/typescript/guide/getting-started",fallback:true});
});
test("blocked or invalid storage cannot choose an unsupported SDK", () => {
  assert.equal(readSDKPreference({getItem(){throw Error("blocked");}}),null);
  assert.equal(readSDKPreference({getItem(){return "rust";}}),null);
  assert.equal(readSDKPreference({getItem(){return "golang";}}),"golang");
});
test("catalog covers historical routes and specific scopes precede ko", () => {
  assert.equal(topics.length,28);assert.equal(new Set(topics.map(x=>x.id)).size,28);
  assert.equal(legacyRoutes.length,52);assert.equal(new Set(legacyRoutes.map(x=>x.route)).size,52);
  assert.equal(legacyRoutes.reduce((n,x)=>n+x.anchors.length,0),430);
  assert.ok(documentationScopes.findIndex(x=>x.key==="ko/golang")<documentationScopes.findIndex(x=>x.key==="ko"));
});

test("catalog validation rejects collisions before navigation is built", async () => {
  const { validateCatalog } = await import("./validate-doc-context.mjs");
  assert.throws(() => validateCatalog([topics[0], topics[0]], []), /Duplicate documentation topic/);
  assert.throws(() => validateCatalog(topics, [legacyRoutes[0], legacyRoutes[0]]), /Duplicate legacy route/);
});
