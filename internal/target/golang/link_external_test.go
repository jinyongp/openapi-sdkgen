package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedExternalLinkClosure(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/fixtures/support-gaps/links/root.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"external_link_test.go": fmt.Sprintf(externalLinkConsumer, "https://target.example.test/v1/items/one%20%2Ftwo")})
}
func TestGeneratedLockedRemoteLinkClosure(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "root.json")
	remoteURL := "https://links.example.test/defs/target.json"
	remote, err := os.ReadFile("../../../test/fixtures/support-gaps/links/target.json")
	if err != nil {
		t.Fatal(err)
	}
	remote = []byte(strings.ReplaceAll(string(remote), "https://target.example.test/v1", "./v2"))
	digest := sha256.Sum256(remote)
	encoded := hex.EncodeToString(digest[:])
	cache := filepath.Join(directory, ".openapi-sdkgen-cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, encoded), remote, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input+".openapi-sdkgen.lock", []byte(`{"version":1,"references":{"`+remoteURL+`":"`+encoded+`"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	source := `{"openapi":"3.2.0","info":{"title":"Remote Link closure","version":"1"},"paths":{"/source":{"get":{"operationId":"getSource","responses":{"200":{"description":"OK","links":{"item":{"operationRef":"` + remoteURL + `#/paths/~1items~1{id}/get","parameters":{"id":"one /two"}}}}}}},"/items/{id}":{"$ref":"` + remoteURL + `#/paths/~1items~1{id}"}}}`
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	options := compiler.CompileOptions{RemoteRefAllowlist: []string{"https://links.example.test"}, Offline: true}
	document, err := compiler.CompileFileWithOptions(input, options)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-W509" {
		t.Fatalf("unmounted back Link should be omitted: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"external_link_test.go": fmt.Sprintf(externalLinkConsumer, "https://links.example.test/defs/v2/items/one%20%2Ftwo")})
	if err := os.Remove(filepath.Join(cache, encoded)); err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.CompileFileWithOptions(input, options); err == nil {
		t.Fatal("missing locked cache accepted")
	}
}

const externalLinkConsumer = `package consumer_test
import("context";"io";"net/http";"reflect";"strings";"testing";sdk "example.test/consumer/api")
type roundTripper func(*http.Request)(*http.Response,error)
func(transport roundTripper)RoundTrip(request *http.Request)(*http.Response,error){return transport(request)}
func TestCompiledExternalLinkTarget(t *testing.T){var seen []string;client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://source.example.test",HTTPClient:&http.Client{Transport:roundTripper(func(request *http.Request)(*http.Response,error){seen=append(seen,request.URL.String());return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":[]string{"application/json"}},Body:io.NopCloser(strings.NewReader("{\"id\":\"one /two\"}")),Request:request},nil})}});if err!=nil{t.Fatal(err)};source,err:=client.Operations.GetSource().Raw(context.Background());if err!=nil{t.Fatal(err)};result,err:=client.Links.GetSource.Item.Follow(source).Do(context.Background());if err!=nil||result.ID!="one /two"{t.Fatal(result,err)};if !reflect.DeepEqual(seen,[]string{"https://source.example.test/source",%q}){t.Fatal("closure/server identity",seen)}}
`
