package typescript

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

func TestMultipartBinaryOutputSchemaCompatibility(t *testing.T) {
	for index, schema := range []string{`true`, `{"type":"string"}`, `{"type":"string","contentEncoding":"binary"}`, `{"type":"object"}`, `{"type":"array"}`} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Binary probe","version":"1"},"paths":{"/frames":{"get":{"operationId":"frames","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"itemSchema":` + schema + `,"itemEncoding":{"contentType":"application/octet-stream"}}}}}}}}}`))
			if err != nil {
				t.Fatal(err)
			}
			output := compileTypeScriptArtifacts(t, document)
			script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);const api=createClient({baseURL:'https://example.test',fetch:async()=>new Response('--b\r\ncontent-type: application/octet-stream\r\n\r\nabc\r\n--b--',{headers:{'content-type':'multipart/mixed; boundary=b'}})});try{const items=[];for await(const item of api.$operations.frames.stream())items.push({binary:item instanceof ArrayBuffer,tag:Object.prototype.toString.call(item),json:JSON.stringify(item)});console.log(JSON.stringify({items}));}catch(error){console.log(JSON.stringify({error:error.code,cause:String(error.cause)}));}`
			result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput()
			if err != nil {
				t.Fatalf("binary output consumer: %v\n%s", err, result)
			}
			var observed struct {
				Items []struct {
					Binary bool `json:"binary"`
				} `json:"items"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(result, &observed); err != nil {
				t.Fatalf("decode result: %v: %s", err, result)
			}
			if index == 0 || index == 3 {
				if len(observed.Items) != 1 || !observed.Items[0].Binary || observed.Error != "" {
					t.Fatalf("binary output lost: %s", result)
				}
			} else if observed.Error != "RESPONSE_DECODE_FAILED" {
				t.Fatalf("binary output violated schema: %s", result)
			}
		})
	}
}

func TestRecursiveMultipartResponseSchemaFacets(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Recursive parts","version":"1"},"paths":{"/recursive":{"get":{"operationId":"recursive","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"itemSchema":{"$ref":"#/components/schemas/Node"}}}}}}}},"components":{"schemas":{"Node":{"anyOf":[{"type":"array","items":{"$ref":"#/components/schemas/Node"}},{"type":"object"}]}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);const api=createClient({baseURL:'https://example.test',fetch:async()=>new Response('--b\r\nContent-Type: multipart/mixed; boundary=n\r\n\r\n--n\r\nX-Long: '+'x'.repeat(9000)+'\r\nContent-Type: application/octet-stream\r\n\r\nabc\r\n--n--\r\n\r\n--b--\r\n',{headers:{'content-type':'multipart/mixed; boundary=b'}})});let count=0;for await(const item of api.$operations.recursive.stream()){count++;if(!Array.isArray(item)||item.length!==1||!(item[0] instanceof ArrayBuffer)||new TextDecoder().decode(item[0])!=='abc')throw new Error('recursive multipart bytes changed');}if(count!==1)throw new Error('recursive item count changed');`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("recursive consumer: %v\n%s", err, result)
	}
}
