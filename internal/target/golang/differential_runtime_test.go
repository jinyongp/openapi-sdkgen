package golang

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
	"openapi-sdkgen/internal/tscheck"
)

// Both actual SDKs consume the original same fixture. Assertions below are
// authored expected HTTP/JSON behavior, not snapshots of either implementation.
func TestGeneratedTypeScriptGoSameHTTPContract(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/typescript/fixtures/runtime-features/client/small-json.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, findings, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(findings) != 0 {
		t.Fatal(err, findings)
	}
	testGeneratedModule(t, plan, map[string]string{"differential_test.go": sameHTTPGoConsumer})
	artifacts, err := (typescript.Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	for _, artifact := range artifacts {
		p := filepath.Join(source, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, artifact.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	options := tscheck.CompilerOptions()
	options["target"] = "ES2022"
	options["module"] = "NodeNext"
	options["moduleResolution"] = "NodeNext"
	options["lib"] = []string{"ES2022", "DOM", "DOM.Iterable"}
	options["outDir"] = "../output"
	options["rootDir"] = "."
	config, _ := json.Marshal(map[string]any{"compilerOptions": options, "include": []string{"**/*.ts"}})
	for name, data := range map[string][]byte{"tsconfig.json": config, "package.json": []byte(`{"type":"module"}`)} {
		if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "../../../test/typescript/node_modules/typescript/lib/tsc.js", "--project", filepath.Join(source, "tsconfig.json"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("TypeScript compile: %v\n%s", err, output)
	}
	output := filepath.Join(root, "output")
	if err := os.WriteFile(filepath.Join(output, "package.json"), []byte(`{"type":"module"}`), 0600); err != nil {
		t.Fatal(err)
	}
	command = exec.CommandContext(ctx, "node", "--input-type=module", "--eval", sameHTTPTypeScriptConsumer, filepath.Join(output, "index.js"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("same fixture independent HTTP oracle: %v\n%s", err, output)
	}
}

const sameHTTPGoConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"strings";"testing";api "example.test/consumer/api";"example.test/consumer/api/types")
type oracleTransport struct{t *testing.T;body string;status,calls int;wantPath,wantAccept string}
func(t *oracleTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{}}
func(t *oracleTransport)Do(r *http.Request,_ types.TransportOptions)(*http.Response,error){t.calls++;if r.URL.String()!="https://size.test"+t.wantPath||r.Method!="GET"||r.Header.Get("Authorization")!="Bearer benchmark"||r.Header.Get("Accept")!=t.wantAccept{t.t.Fatal(r.URL,r.Method,r.Header)};return &http.Response{StatusCode:t.status,Header:http.Header{"Content-Type":{"application/json"}},Body:io.NopCloser(strings.NewReader(t.body))},nil}
func TestIndependentHTTPOracle(t *testing.T){for _,scenario:=range []struct{name,body,id,path,accept string;status int;raw,invalid,requestInvalid bool}{
 {name:"ordinary",body:"{\"id\":\"one\",\"title\":\"One\"}",id:"one",path:"/items/one",status:200},
 {name:"explicit-accept",accept:"application/json",body:"{\"id\":\"one\",\"title\":\"One\"}",id:"one",path:"/items/one",status:200},
 {name:"reserved-header",requestInvalid:true,id:"one",path:"/items/one",body:"{\"id\":\"one\",\"title\":\"One\"}",status:200},
 {name:"raw",body:"{\"id\":\"one\",\"title\":\"One\"}",id:"one",path:"/items/one",status:200,raw:true},
 {name:"unicode",body:"{\"id\":\"one\",\"title\":\"One\"}",id:"한 글/%",path:"/items/%ED%95%9C%20%EA%B8%80%2F%25",status:200},
 {name:"missing-required-output",body:"{\"title\":\"One\"}",id:"one",path:"/items/one",status:200,invalid:true},
 {name:"malformed-json",body:"{bad",id:"one",path:"/items/one",status:200,invalid:true},
 {name:"declared-error",body:"{\"error\":\"bad\"}",id:"one",path:"/items/one",status:400},
}{t.Run(scenario.name,func(t *testing.T){transport:=&oracleTransport{t:t,body:scenario.body,status:scenario.status,wantPath:scenario.path,wantAccept:scenario.accept};client,err:=api.NewClient(api.ClientOptions{BaseURL:"https://size.test",Authorization:types.Some("Bearer benchmark"),Transport:transport});if err!=nil{t.Fatal(err)};call:=client.Operations.GetItem(api.GetItemInput{Path:api.GetItemPath{ID:scenario.id}});if scenario.requestInvalid{call=call.WithOptions(types.RequestOptions{Headers:http.Header{"authorization":{"unexpected"}}})};if scenario.accept!=""{call=call.WithOptions(types.RequestOptions{Accept:types.Some(scenario.accept)})};ctx:=context.Background();if scenario.raw{raw,err:=call.Raw(ctx);if err!=nil||raw.Status!=200{t.Fatal(raw,err)};id,_:=raw.Data.Get("id");text,_:=id.String();if text!="one"{t.Fatal(raw.Data)}}else{got,err:=call.Do(ctx);if scenario.requestInvalid{var failure *api.APIError;if !errors.As(err,&failure)||failure.Code!="REQUEST_ENCODE_FAILED"{t.Fatal(err)}}else if scenario.invalid{var failure *api.APIError;if !errors.As(err,&failure)||failure.Code!="RESPONSE_DECODE_FAILED"{t.Fatal(err)}}else if scenario.status==400{var failure *api.APIError;if !errors.As(err,&failure)||failure.Status!=400{t.Fatal(err)};detail,_:=failure.Data.Get("error");text,_:=detail.String();if text!="bad"{t.Fatal(failure.Data)}}else if err!=nil{t.Fatal(err)}else{title,_:=got.Title.Get();if got.ID!="one"||title!="One"{t.Fatal(got)}}};wantCalls:=1;if scenario.requestInvalid{wantCalls=0};if transport.calls!=wantCalls{t.Fatal(transport.calls)}})}}
`

const sameHTTPTypeScriptConsumer = `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const {createClient}=await import(pathToFileURL(process.argv[1]));
for(const test of [{name:'ordinary',id:'one',path:'/items/one',body:'{"id":"one","title":"One"}',status:200},{name:'explicit-accept',accept:'application/json',id:'one',path:'/items/one',body:'{"id":"one","title":"One"}',status:200},{name:'reserved-header',requestInvalid:true,id:'one',path:'/items/one',body:'{"id":"one","title":"One"}',status:200},{name:'raw',id:'one',path:'/items/one',body:'{"id":"one","title":"One"}',status:200,raw:true},{name:'unicode',id:'한 글/%',path:'/items/%ED%95%9C%20%EA%B8%80%2F%25',body:'{"id":"one","title":"One"}',status:200},{name:'missing-required-output',id:'one',path:'/items/one',body:'{"title":"One"}',status:200,invalid:true},{name:'malformed-json',id:'one',path:'/items/one',body:'{bad',status:200,invalid:true},{name:'declared-error',id:'one',path:'/items/one',body:'{"error":"bad"}',status:400}]){
 let calls=0;const api=createClient({baseURL:'https://size.test',authorization:'Bearer benchmark',fetch:async(url,init)=>{calls++;assert.equal(String(url),'https://size.test'+test.path);assert.equal(init.method,'GET');const headers=new Headers(init.headers);assert.equal(headers.get('authorization'),'Bearer benchmark');assert.equal(headers.get('accept'),test.accept??null);return new Response(test.body,{status:test.status,headers:{'Content-Type':'application/json'}})}});
 const input={path:{id:test.id}},options=test.requestInvalid?{headers:{authorization:"unexpected"}}:test.accept?{accept:test.accept}:{};if(test.requestInvalid)await assert.rejects(api.$operations.getItem(input,options),{code:'REQUEST_ENCODE_FAILED'});else if(test.invalid)await assert.rejects(api.$operations.getItem(input,options),{code:'RESPONSE_DECODE_FAILED'});else if(test.status===400)await assert.rejects(api.$operations.getItem(input,options),{status:400});else if(test.raw){const raw=await api.$operations.getItem.raw(input,options);assert.equal(raw.status,200);assert.deepEqual(raw.data,{id:'one',title:'One'})}else assert.deepEqual(await api.$operations.getItem(input,options),{id:'one',title:'One'});assert.equal(calls,test.requestInvalid?0:1,test.name);
}`
