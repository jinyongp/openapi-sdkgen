package typescript

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestServerSharedAliasesRetainEachReceivingContract(t *testing.T) {
	input, err := os.ReadFile("../../../test/typescript/fixtures/runtime-features/server/schema-aliases.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	target := Generator{}
	plan, _, err := target.Prepare(document, options)
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := plan.Value("typescript")
	source := prepared.(*sourcePlan)
	before := schemaProgramsSnapshot(t, source.serverSchemaPrograms)
	artifacts, err := target.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, schemaProgramsSnapshot(t, source.serverSchemaPrograms)) {
		t.Fatal("receiving file emission mutated the shared cache")
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createCallbackHandlers} from './server/callbacks.js'; import {createWebhookRouter} from './server/webhooks.js'; void [createCallbackHandlers,createWebhookRouter];`)
	script := `import assert from 'node:assert/strict'; import path from 'node:path'; import {pathToFileURL} from 'node:url';
const base=process.argv[1];
const {createCallbackHandlers}=await import(pathToFileURL(path.join(base,'server/callbacks.js')));
const {createWebhookRouter}=await import(pathToFileURL(path.join(base,'server/webhooks.js')));
const expression='{$request.body#/callbackURL}';
const echo=async ({body})=>({status:200,body});
const callbacks=createCallbackHandlers({callbacks:{subscribe:{number:{[expression]:{POST:echo}}}},componentCallbacks:{text:{[expression]:{POST:echo}}}});
const router=createWebhookRouter({number:{POST:echo},text:{POST:echo}},{routes:{number:'/number',text:'/text'}});
const endpoints=[['number',callbacks.callbacks.subscribe.number[expression].POST],['text',callbacks.componentCallbacks.text[expression].POST],['number',router],['text',router]];
for(const [kind,endpoint] of endpoints){
 const value=kind==='number'?3:'ok';
 const request=(body,tag=kind)=>new Request('https://host.test/'+kind+'?tag='+tag,{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});
 const response=await endpoint.fetch(request({value}));
 assert.equal(response.status,200); assert.deepEqual(await response.json(),{value});
 assert.equal((await endpoint.fetch(request({value:kind==='number'?'ok':3}))).status,400);
 assert.equal((await endpoint.fetch(request({value},kind==='number'?'text':'number'))).status,400);
}`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Clean(output)).CombinedOutput(); err != nil {
		t.Fatalf("receiving contracts share the wrong alias: %v\n%s", err, result)
	}
}
