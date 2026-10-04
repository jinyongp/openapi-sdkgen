package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestRuntimeFeatureHandlerCompositionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Handlers","version":"1"},"components":{"schemas":{"Tiny":{"type":"object","required":["id","tags"],"properties":{"id":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}}},"additionalProperties":false},"Asserted":{"$vocabulary":{"https://json-schema.org/draft/2020-12/vocab/format-assertion":true},"type":"string","format":"uuid"}},"securitySchemes":{"Token":{"type":"http","scheme":"bearer"}}},"paths":{"/tiny":{"post":{"operationId":"tiny","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Tiny"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Tiny"}}}}}}},"/asserted":{"get":{"operationId":"asserted","security":[{"Token":[]}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Asserted"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	plan, _, _, err := prepareClientSourcePlanWithCoverage(document, generator.Options{}, "")
	if err != nil {
		t.Fatal(err)
	}
	provider := func(route string) string {
		return filepath.ToSlash(operationExecutionArtifactPath(operationModulePlan{path: plan.modules.operationByRoute[route]}))
	}
	for _, artifact := range artifacts {
		if strings.HasSuffix(artifact.Path, "/xml-codec.ts") || strings.HasSuffix(artifact.Path, "/security-basic.ts") {
			t.Fatalf("unneeded generated artifact %s", artifact.Path)
		}
	}
	// The negative control is an independent full-capability composition made
	// from this same source basis, never the shipping default client.
	full, err := emitRuntimeTemplateArtifacts()
	if err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool)
	for _, artifact := range artifacts {
		present[artifact.Path] = true
	}
	for _, artifact := range full {
		if !present[artifact.Path] {
			artifacts = append(artifacts, artifact)
		}
	}
	artifacts = append(artifacts, Artifact{Path: "control.ts", Data: []byte("export { fullRequestServices } from './internal/runtime/compatibility/http-codecs.js'\n")})
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	ts, err := filepath.Abs("../../../test/typescript/node_modules/typescript-6/lib/typescript.js")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import fs from 'node:fs';import path from 'node:path';import {pathToFileURL} from 'node:url';
const ts=(await import(pathToFileURL(process.argv[2]))).default;
function graph(entry){const seen=new Set();function walk(file){file=path.resolve(process.argv[1],file);if(seen.has(file))return;assert.ok(fs.existsSync(file),'missing candidate dependency '+file);seen.add(file);const source=ts.createSourceFile(file,fs.readFileSync(file,'utf8'),ts.ScriptTarget.Latest,true,ts.ScriptKind.JS);function visit(node){let specifier;if(ts.isImportDeclaration(node)||ts.isExportDeclaration(node))specifier=node.moduleSpecifier;else if(ts.isCallExpression(node)&&node.expression.kind===ts.SyntaxKind.ImportKeyword)specifier=node.arguments[0];if(specifier!==undefined){assert.ok(ts.isStringLiteral(specifier),'nonliteral generated import '+file);if(specifier.text.startsWith('.'))walk(path.resolve(path.dirname(file),specifier.text));}ts.forEachChild(node,visit);}visit(source);}walk(entry);return new Set([...seen].map(file=>path.basename(file)));}
const tinyGraph=graph(process.argv[3]),assertedGraph=graph(process.argv[4]),fullGraph=graph('control.js'),rootGraph=graph('internal/client/factory.js');
for(const module of ['http-advanced.js','http-core.js','http-multipart-request.js','http-multipart-response.js','xml-codec.js','wire-handlers.js','wire-format.js','wire-format-date.js','wire-dynamic.js','wire-content.js','stream-sse.js','security-basic.js','security-oauth.js']){assert.equal(tinyGraph.has(module),false,'unneeded tiny handler '+module);assert.equal(fullGraph.has(module),true,'negative control does not detect full capability '+module);}
assert.equal(assertedGraph.has('wire-format-uuid.js'),true);assert.equal(assertedGraph.has('security-bearer.js'),true);assert.equal(assertedGraph.has('wire-format-date.js'),false);assert.equal(assertedGraph.has('security-basic.js'),false);
assert.equal(rootGraph.has('xml-codec.js'),false);assert.equal(rootGraph.has('security-basic.js'),false);assert.equal(rootGraph.has('wire-format-uuid.js'),true);assert.equal(rootGraph.has('security-bearer.js'),true);
const load=file=>import(pathToFileURL(process.argv[1]+'/'+file));const root=await load('index.js'),selective=await load('selective/index.js');const prepared=await selective.loadOperations([selective.operations.tiny,selective.operations.asserted]);
for(const create of [root.createClient,options=>selective.createClient({...options,operations:prepared})]){
 const requests=[];let invalid=false;const api=create({baseURL:'https://api.test',securityProvider:()=>({Token:{kind:'http-bearer',token:'secret'}}),fetch:async(url,init)=>{requests.push({url,init});if(new URL(url).pathname==='/asserted'){assert.equal(new Headers(init.headers).get('authorization'),'Bearer secret');return Response.json(invalid?'invalid':'f47ac10b-58cc-4372-a567-0e02b2c3d479');}return Response.json({id:'one',tags:['one']});}});
 assert.deepEqual(await api.$operations.tiny({body:{id:'one',tags:['one']}}),{id:'one',tags:['one']});assert.deepEqual(JSON.parse(requests[0].init.body),{id:'one',tags:['one']});
 await assert.rejects(api.$operations.tiny({body:{id:'one',tags:[3]}}),{code:'REQUEST_ENCODE_FAILED'});
 assert.equal(await api.$operations.asserted(),'f47ac10b-58cc-4372-a567-0e02b2c3d479');invalid=true;await assert.rejects(api.$operations.asserted(),{code:'RESPONSE_DECODE_FAILED'});
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, ts, strings.TrimSuffix(provider("POST /tiny"), ".ts")+".js", strings.TrimSuffix(provider("GET /asserted"), ".ts")+".js").CombinedOutput(); err != nil {
		t.Fatalf("actual generated handler graph and behavior: %v\n%s", err, result)
	}
}
