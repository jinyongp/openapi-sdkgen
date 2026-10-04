package typescript

import (
	"os/exec"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestPlainDecodedDTORuntimeRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
"openapi":"3.2.0","info":{"title":"Plain DTO","version":"1"},
"components":{"schemas":{"Entry":{"type":"object","xml":{"name":"entry"},"required":["id","profile"],"properties":{"id":{"type":"string"},"profile":{"type":"object","required":["city"],"properties":{"city":{"type":"string"}}},"tags":{"type":"array","items":{"type":"string"}}}}}},
"paths":{
"/json":{"get":{"operationId":"json","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Entry"}}}}}}},
"/xml":{"get":{"operationId":"xml","responses":{"200":{"description":"ok","content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Entry"}}}}}}},
"/parts":{"get":{"operationId":"parts","responses":{"200":{"description":"ok","content":{"multipart/mixed":{"schema":{"type":"array","items":{"$ref":"#/components/schemas/Entry"}},"itemEncoding":{"contentType":"application/json"}}}}}}},
"/stream":{"get":{"operationId":"stream","responses":{"200":{"description":"ok","content":{"application/x-ndjson":{"itemSchema":{"$ref":"#/components/schemas/Entry"}}}}}}},
"/error":{"get":{"operationId":"error","responses":{"400":{"description":"error","content":{"application/json":{"schema":{"type":"object","properties":{"code":{"type":"string"},"details":{"$ref":"#/components/schemas/Entry"}}}}}}}}},
"/pages":{"get":{"operationId":"pages","parameters":[{"name":"cursor","in":"query","schema":{"type":"string"}},{"name":"limit","in":"query","schema":{"type":"integer","minimum":1}}],"x-pagination":"cursor","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/Entry"}},"pagination":{"type":"object","properties":{"nextCursor":{"type":["string","null"]}}}}}}}}}}}
},
"webhooks":{"entry":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Entry"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Entry"}}}}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := registry.Resolve([]string{"server"})
	options.Clients = map[string]generator.Client{"plain": {Selection: &generator.Selection{Operations: []string{"json", "xml", "parts", "stream", "error", "pages"}}}}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	script := `
import assert from 'node:assert/strict'; import {pathToFileURL} from 'node:url';
const load = file => import(pathToFileURL(process.argv[1] + '/' + file));
const {createClient} = await load('index.js'), named = (await load('clients/plain/index.js')).createClient;
const selective = await load('selective/index.js');
const prepared = await selective.loadOperations(['json','xml','parts','stream','error','pages'].map(name=>selective.operations[name]));
const entry = {id:'one',profile:{city:'Seoul'},tags:['a']};
function plain(value) {
  if (value === null || typeof value !== 'object') return;
  assert.equal(Object.getPrototypeOf(value),Array.isArray(value)?Array.prototype:Object.prototype);
  if(!Array.isArray(value)) assert.equal(value instanceof Object,true);
  for(const child of Object.values(value)) plain(child);
}
const configuration = {baseURL:'https://api.test',fetch:async url=>{
  const path = new URL(url).pathname;
  if(path === '/xml') return new Response('<entry><id>one</id><profile><city>Seoul</city></profile><tags>a</tags></entry>',{headers:{'content-type':'application/xml'}});
  if(path === '/parts') return new Response('--part\r\nContent-Type: application/json\r\n\r\n'+JSON.stringify(entry)+'\r\n--part--\r\n',{headers:{'content-type':'multipart/mixed; boundary=part'}});
  if(path === '/stream') return new Response(JSON.stringify(entry)+'\n',{headers:{'content-type':'application/x-ndjson'}});
  if(path === '/error') return Response.json({code:'BAD',details:entry},{status:400});
  if(path === '/pages') return Response.json({items:[entry],pagination:{nextCursor:null}});
  return Response.json(entry);
}};
for(const create of [createClient,named,options=>selective.createClient({...options,operations:prepared})]) {
  const api=create(configuration);
  const json=await api.$operations.json(); plain(json); assert.deepEqual(json,entry);
  plain((await api.$operations.json.raw()).data);
  plain(await api.$operations.xml());
  plain(await api.$operations.parts());
  const stream=[];for await(const item of api.$operations.stream.stream()){plain(item);stream.push(item);}assert.deepEqual(stream,[entry]);
  const pages=[];for await(const item of api.$operations.pages.paginate()){plain(item);pages.push(item);}assert.deepEqual(pages,[entry]);
  await assert.rejects(api.$operations.error(),error=>{plain(error.data);plain(error.details);assert.deepEqual(error.details,entry);return true;});
}
const {createWebhookRouter}=await load('server/webhooks.js');
let received;
const router=createWebhookRouter({entry:{POST:async({body})=>{plain(body);received=body;return {status:200,body};}}},{routes:{entry:'/entry'}});
assert.equal((await router.fetch(new Request('https://host.test/entry',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(entry)}))).status,200);
assert.deepEqual(received,entry);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("execute generated plain DTO SDK: %v\n%s", err, result)
	}
}
