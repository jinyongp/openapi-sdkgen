package typescript

import (
	"fmt"
	"os/exec"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestWireContractStreamFramingRegression(t *testing.T) {
	registry, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := registry.Resolve([]string{"server"})
	for _, media := range []string{"application/x-ndjson", "application/json-seq", "multipart/mixed"} {
		t.Run(media, func(t *testing.T) {
			encoding := ""
			if media == "multipart/mixed" {
				encoding = `,"itemEncoding":{"contentType":"application/json"}`
			}
			content := `{"` + media + `":{"itemSchema":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}` + encoding + `}}`
			document, err := sdkgen.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Framing regression","version":"1"},"paths":{"/events":{"get":{"operationId":"events","responses":{"200":{"description":"ok","content":` + content + `}}}}},"webhooks":{"events":{"post":{"requestBody":{"required":true,"content":` + content + `},"responses":{"204":{"description":"ok"}}}}}}`))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := (Generator{}).Generate(document, options)
			if err != nil {
				t.Fatal(err)
			}
			output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; import {createWebhookRouter} from './server/webhooks.js'; void createClient; void createWebhookRouter;`)
			script := fmt.Sprintf(`
import assert from 'node:assert/strict'; import {pathToFileURL} from 'node:url';
const load = file => import(pathToFileURL(process.argv[1] + '/' + file));
const {createClient} = await load('index.js'), {createWebhookRouter} = await load('server/webhooks.js');
const media = %q, contentType = media === 'multipart/mixed' ? media + '; boundary=ababab' : media;
const items = [{event_id:'한글😀\r\ntext'}, {event_id:'last'}], expected = items;
const json = items.map(item=>JSON.stringify(item));
const wire = media === 'multipart/mixed' ? '--ababab\r\nContent-Type: application/json\r\n\r\n' + json.join('\r\n--ababab\r\nContent-Type: application/json\r\n\r\n') + '\r\n--ababab--\r\n' : json.map(value=>(media === 'application/json-seq' ? '\u001e' : '') + value + '\r\n').join('');
const bytes = new TextEncoder().encode(wire);
function chunked(size) { let offset=0; return new ReadableStream({pull(controller) { if(offset>=bytes.length) {controller.close(); return;} controller.enqueue(bytes.subarray(offset,offset+size)); offset+=size;}}, {highWaterMark:0}); }
for (const size of [1,2,3,64,65536]) {
  const body = chunked(size), client = createClient({baseURL:'https://api.test',fetch:async()=>new Response(body,{headers:{'content-type':contentType}})});
  const seen=[]; for await (const item of client.$operations.events.stream()) seen.push(item);
  assert.deepEqual(JSON.parse(JSON.stringify(seen)),expected); assert.equal(body.locked,false);
  const received=[];
  const router = createWebhookRouter({events:{POST:async({body})=>{for await(const item of body) received.push(item);return {status:204};}}},{routes:{events:'/events'},maxStreamFrameBytes:1024});
  const inbound=chunked(size);
  assert.equal((await router.fetch(new Request('https://host.test/events',{method:'POST',headers:{'content-type':contentType},body:inbound,duplex:'half'}))).status,204);
  assert.deepEqual(JSON.parse(JSON.stringify(received)),expected); assert.equal(inbound.locked,false);
}
`, media)
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
				t.Fatalf("execute generated framing SDK: %v\n%s", err, result)
			}
		})
	}
}
