package golang

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/output"
)

const healthDocument = `{"openapi":"3.1.0","info":{"title":"Health","version":"1"},"paths":{"/health":{"get":{"operationId":"getHealth","responses":{"200":{"description":"Healthy","content":{"application/json":{"schema":{"type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}}}}}}}}}`

func healthPlan(t *testing.T) (*ir.Document, generator.Plan) {
	t.Helper()
	document, err := compiler.Compile([]byte(healthDocument))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v, %v", err, diagnostics)
	}
	return document, plan
}

func TestPreparedSourceIsIndependentAndDeterministic(t *testing.T) {
	document, plan := healthPlan(t)
	target := Generator{}
	first, err := target.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	document.Operations[0].Path = "/mutated"
	document.Operations[0].OperationID = "mutated"
	document.Raw["paths"] = nil
	second, err := target.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("prepared output retained mutable input")
	}
	collector := &artifactCollector{}
	if err := target.EmitTo(plan, collector); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, collector.artifacts) {
		t.Fatal("Emit and EmitTo differ")
	}
	_, fresh := healthPlan(t)
	third, err := target.Emit(fresh)
	if err != nil || !reflect.DeepEqual(first, third) {
		t.Fatalf("preparation is nondeterministic: %v", err)
	}
	first[0].Data[0] = '!'
	fourth, err := target.Emit(plan)
	if err != nil || !reflect.DeepEqual(second, fourth) {
		t.Fatalf("artifact ownership leaked: %v", err)
	}
}

func TestWholeDocumentNamesRemainStable(t *testing.T) {
	document := &ir.Document{Operations: []ir.Operation{
		{RouteKey: "GET /a", Method: "GET", Path: "/a", OperationID: "get-health"},
		{RouteKey: "GET /b", Method: "GET", Path: "/b", OperationID: "get_health"},
		{RouteKey: "GET /c", Method: "GET", Path: "/c", OperationID: "getHealthInput"},
		{RouteKey: "GET /d", Method: "GET", Path: "/d", OperationID: "operations"},
	}}
	first, err := planNames(document)
	if err != nil {
		t.Fatal(err)
	}
	document.Operations[0], document.Operations[3] = document.Operations[3], document.Operations[0]
	second, err := planNames(document)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("names depend on document iteration order")
	}
	if first.operations["GET /a"] != "GetHealth" || first.operations["GET /b"] != "GetHealth_"+nameDigest("GET /b") || first.operations["GET /c"] == "GetHealthInput" || first.operations["GET /d"] == "Operations" {
		t.Fatalf("unresolved public name collisions: %#v", first.operations)
	}
}

func nameDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest)[:8]
}

func TestNameCollisionExtendsOriginalPathDigest(t *testing.T) {
	identity := "#/paths/~1value/get"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	used := map[string]bool{"Reserved": true, "Reserved_" + digest[:8]: true}
	name, err := reservePublicName(used, "reserved", identity, "")
	if err != nil || name != "Reserved_"+digest[:12] {
		t.Fatalf("hash extension: %s %v", name, err)
	}
	for _, raw := range []string{"type", "123"} {
		name, err := reservePublicName(make(map[string]bool), raw, identity, "")
		if err != nil || name != exportedName(raw)+"_"+digest[:8] {
			t.Fatalf("unsafe name: %s %v", name, err)
		}
	}
}

type failingSink struct {
	calls   int
	failure error
}

func (sink *failingSink) WriteArtifact(generator.Artifact) error { sink.calls++; return sink.failure }

func TestEmitStopsAtSinkFailure(t *testing.T) {
	_, plan := healthPlan(t)
	failure := errors.New("disk unavailable")
	sink := &failingSink{failure: failure}
	if err := (Generator{}).EmitTo(plan, sink); !errors.Is(err, failure) || sink.calls != 1 {
		t.Fatalf("sink failure: %v (%d writes)", err, sink.calls)
	}
	if err := (Generator{}).EmitTo(generator.NewPlan("typescript", 1), sink); err == nil {
		t.Fatal("accepted foreign plan")
	}
	if err := (Generator{}).EmitTo(plan, nil); err == nil {
		t.Fatal("accepted nil sink")
	}
}

func TestPreparedFileImportsRejectCyclesAndMissingPackages(t *testing.T) {
	for _, files := range [][]filePlan{
		{{Path: "a/a.go", Package: "a", Imports: []importPlan{{Path: "example.test/api/b"}}}, {Path: "b/b.go", Package: "b", Imports: []importPlan{{Path: "example.test/api/a"}}}},
		{{Path: "a/a.go", Package: "a", Imports: []importPlan{{Path: "example.test/api/missing"}}}},
		{{Path: "a/a.go", Package: "a"}, {Path: "a/b.go", Package: "b"}},
	} {
		if err := validateFileImports("example.test/api", files); err == nil {
			t.Fatalf("accepted invalid package graph: %#v", files)
		}
	}
}

func TestFormattingFailureRollsBackStagedGoArtifacts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "api")
	if err := output.PublishArtifacts(root, []generator.Artifact{{Path: "user-base.go", Data: []byte("package api\n")}}, false, nil); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, output.ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	plan := generator.NewPlan("golang", sourcePlan{Files: []filePlan{
		{Path: "internal/test/source.go", Package: "test", Template: "templates/prepared.go.tmpl", Prepared: "type Marker struct{}"},
		{Path: "broken.go", Package: "invalid package", Template: "templates/prepared.go.tmpl"},
	}})
	err = output.StreamArtifacts(root, true, nil, func(sink generator.ArtifactSink) error { return (Generator{}).EmitTo(plan, sink) })
	if err == nil {
		t.Fatal("accepted invalid generated source")
	}
	after, err := os.ReadFile(filepath.Join(root, output.ManifestName))
	if err != nil || string(before) != string(after) {
		t.Fatalf("failed emission changed manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial Go emission published: %v", err)
	}
}

func TestGeneratedHealthConsumer(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumerTest})
}

func testGeneratedModule(t *testing.T, plan generator.Plan, files map[string]string) string {
	t.Helper()
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, artifact := range artifacts {
		file := filepath.Join(root, "api", filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, artifact.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files["go.mod"] = "module example.test/consumer\n\ngo 1.27.0\n"
	modFlag := "-mod=readonly"
	for _, artifact := range artifacts {
		if artifact.Path == "internal/transport/body.go" {
			files["go.mod"] += "\nrequire (\n github.com/andybalholm/brotli v1.2.6\n github.com/klauspost/compress v1.20.3\n)\n"
			modFlag = "-mod=mod"
		}
		if artifact.Path == "internal/schema/regex.go" {
			files["go.mod"] += "\nrequire github.com/go-quickjs/go-quickjs v0.27.0\n"
			modFlag = "-mod=mod"
		}
		if artifact.Path == "internal/transport/url.go" {
			files["go.mod"] += "\nrequire (\n github.com/nlnwa/whatwg-url v0.6.2\n golang.org/x/net v0.61.0\n golang.org/x/text v0.43.0\n github.com/bits-and-blooms/bitset v1.20.0\n)\n"
			modFlag = "-mod=mod"
		}
	}
	for name, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", modFlag, "-race", "-timeout=45s", "./...")
	command.WaitDelay = 5 * time.Second
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated consumer: %v\n%s", err, output)
	}
	return root
}

const consumerTest = `package consumer_test
import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "net/http/httptest"
 "testing"
 sdk "example.test/consumer/api"
)
func TestHealth(t *testing.T) {
 var uninitialized sdk.Client
 if _,err:=uninitialized.Operations.GetHealth().Do(context.Background());err==nil{t.Fatal("uninitialized client accepted")}else{var detail *sdk.SDKError;if !errors.As(err,&detail)||detail.Code!="OPERATION_NOT_PREPARED"{t.Fatal("uninitialized client failure",err)}}
 server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if r.URL.Path != "/v1/health" || r.Method != "GET" || r.Header.Get("Accept") != "" { t.Errorf("request: %s %s %v",r.Method,r.URL,r.Header) }
  w.Header().Set("Content-Type","application/json")
  io.WriteString(w,"{\"ok\":true}")
 }))
 defer server.Close()
 api,err := sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL+"/v1"})
 if err != nil { t.Fatal(err) }
 value,err := api.Operations.GetHealth().Do(context.Background())
 if err != nil { t.Fatal(err) }
 data,err := json.Marshal(value)
 if err != nil || string(data) != "{\"ok\":true}" { t.Fatalf("response: %s %v",data,err) }
 response,err := api.Operations.GetHealth().Raw(context.Background())
 if err != nil { t.Fatal(err) }
 data = response.Bytes()
 if string(data) != "{\"ok\":true}" || response.OriginalResponse.Body!=http.NoBody { t.Fatalf("raw: %s",data) }
 ctx,cancel := context.WithCancel(context.Background()); cancel()
 if _,err := api.Operations.GetHealth().Do(ctx); err == nil { t.Fatal("ignored cancellation") }
}
`
