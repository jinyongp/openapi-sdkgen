package typescript

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestWireContractSecurityURIRegression(t *testing.T) {
	registry, err := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server", "metadata"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, version, key, identity, declarations string
	}{
		{"name", "3.2.0", "key", "key", ""},
		{"fragment", "3.2.0", "#/components/securitySchemes/key", "key", ""},
		{"percent fragment", "3.2.0", "#%2Fcomponents%2FsecuritySchemes%2Fkey", "key", ""},
		{"older name", "3.1.0", "key", "key", ""},
		{"alias name", "3.2.0", "alias", "alias", `,"alias":{"$ref":"#/components/securitySchemes/key"}`},
		{"alias fragment", "3.2.0", "#/components/securitySchemes/alias", "alias", `,"alias":{"$ref":"#/components/securitySchemes/key"}`},
		{"escaped pointer", "3.2.0", "#/components/securitySchemes/slash~1tilde~0", "slash/tilde~", `,"slash/tilde~":{"$ref":"#/components/securitySchemes/key"}`},
		{"literal percent", "3.2.0", "#/components/securitySchemes/a%252Fb", "a%2Fb", `,"a%2Fb":{"$ref":"#/components/securitySchemes/key"}`},
		{"component name precedence", "3.2.0", "#/components/securitySchemes/key", "#/components/securitySchemes/key", `,"#/components/securitySchemes/key":{"type":"apiKey","in":"header","name":"X-Key"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, _ := json.Marshal(test.key)
			security := `[{` + string(key) + `:[]}]`
			document, err := sdkgen.Compile([]byte(`{"openapi":"` + test.version + `","info":{"title":"Security URI","version":"1"},"components":{"securitySchemes":{"key":{"type":"apiKey","in":"header","name":"X-Key"}` + test.declarations + `}},"security":` + security + `,"paths":{"/secure":{"get":{"operationId":"secure","responses":{"204":{"description":"ok"}}}},"/operation":{"get":{"operationId":"operation","security":` + security + `,"responses":{"204":{"description":"ok"}}}},"/open":{"get":{"operationId":"open","security":[],"responses":{"204":{"description":"ok"}}}}},"webhooks":{"secure":{"get":{"responses":{"204":{"description":"ok"}}}},"open":{"get":{"security":[],"responses":{"204":{"description":"ok"}}}}}}`))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := (Generator{}).Generate(document, options)
			if err != nil {
				t.Fatal(err)
			}
			output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; import {createWebhookRouter} from './server/webhooks.js'; void createClient; void createWebhookRouter;`)
			script := fmt.Sprintf(`
import {pathToFileURL} from "node:url";
import assert from "node:assert/strict";
const load = file => import(pathToFileURL(process.argv[1] + "/" + file));
const {createClient} = await load("index.js");
const {createWebhookRouter} = await load("server/webhooks.js");
const {openapi} = await load("metadata.js");
const key = %q, identity = %q;
assert.deepEqual(openapi.document.security, [{[key]:[]}]);
assert.deepEqual(openapi.document.paths['/operation'].get.security, [{[key]:[]}]);
assert.deepEqual(openapi.document.paths['/open'].get.security, []);
if (identity === 'alias' || identity === 'slash/tilde~' || identity === 'a%%2Fb') assert.equal(openapi.document.components.securitySchemes[identity].$ref, '#/components/securitySchemes/key');
let providers = 0, requests = 0;
const client = createClient({baseURL:'https://api.test',securityProvider: ({requirement}) => {
  providers++;
  assert.equal(requirement.id, key);
  assert.equal(requirement.schemes[0].name, identity);
  return {[identity]:{kind:'api-key',value:'secret'}};
},fetch: async (url, init) => {
  requests++;
  assert.equal(new Headers(init.headers).get('x-key'), new URL(url).pathname === '/open' ? null : 'secret');
  return new Response(null,{status:204});
}});
await client.$operations.secure(); await client.$operations.operation(); await client.$operations.open();
assert.equal(providers,2); assert.equal(requests,3);
let authentications = 0;
const router = createWebhookRouter({secure:{GET: async () => ({status:204})},open:{GET: async () => ({status:204})}}, {
  routes:{secure:'/secure',open:'/open'}, authenticate: ({security,securityCandidates}) => {
    authentications++;
    assert.deepEqual(security,[{[key]:[]}]);
    assert.deepEqual(Object.keys(securityCandidates),[identity]);
    assert.equal(securityCandidates[identity].scheme,identity);
    if (securityCandidates[identity].value !== 'secret') return new Response('denied',{status:401});
  }
});
assert.equal((await router.fetch(new Request('https://host.test/secure',{headers:{'x-key':'secret'}}))).status,204);
assert.equal((await router.fetch(new Request('https://host.test/secure'))).status,401);
assert.equal((await router.fetch(new Request('https://host.test/open'))).status,204);
assert.equal(authentications,2);
`, test.key, test.identity)
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
				t.Fatalf("execute security URI SDK: %v\n%s", err, result)
			}
			// A synthetic target entry uses the same IR kernel even without the
			// compiler's populated registries and effective-operation annotations.
			document.SecuritySchemes = nil
			for index := range document.Operations {
				document.Operations[index].Parameters = nil
			}
			if _, err := (Generator{}).Generate(document, options); err != nil {
				t.Fatalf("synthetic security entry: %v", err)
			}
		})
	}
}

func TestSecurityURIRejectsInvalidTargetsWithoutNetwork(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"type":"apiKey","in":"header","name":"X-Key"}`)
	}))
	defer server.Close()
	for _, key := range []string{"#/components/securitySchemes/missing", "#/info", "./key", server.URL + "/key", "#/components/securitySchemes/key%XX", "#/components/securitySchemes/key~2", "cycle"} {
		t.Run(key, func(t *testing.T) {
			quoted, _ := json.Marshal(key)
			cycle := ""
			if key == "cycle" {
				cycle = `,"cycle":{"$ref":"#/components/securitySchemes/cycle"}`
			}
			document, err := sdkgen.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Invalid Security URI","version":"1"},"components":{"securitySchemes":{"key":{"type":"apiKey","in":"header","name":"X-Key"}` + cycle + `}},"paths":{"/secure":{"get":{"operationId":"secure","security":[{` + string(quoted) + `:[]}],"responses":{"204":{"description":"ok"}}}}}}`))
			if err != nil {
				// The compiler detects a cyclic Reference Object before IR building.
				if key == "cycle" && strings.Contains(err.Error(), "cycl") {
					return
				}
				t.Fatal(err)
			}
			if _, err := SourceArtifacts(document); err == nil {
				t.Fatal("unsupported security target accepted")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("Security Requirement URI fetched another document")
	}
	// A URI requirement stays unavailable before OpenAPI 3.2.
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Old version","version":"1"},"components":{"securitySchemes":{"key":{"type":"apiKey","in":"header","name":"X-Key"}}},"paths":{"/secure":{"get":{"operationId":"secure","security":[{"#/components/securitySchemes/key":[]}],"responses":{"204":{"description":"ok"}}}}}}`))
	if err == nil && len(document.SemanticRestrictions) == 0 {
		t.Fatal("pre-3.2 URI requirement accepted without its compiler restriction")
	}
}
