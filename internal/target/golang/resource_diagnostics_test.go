package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func requireResourceWarnings(t *testing.T, findings []diagnostic.Diagnostic, count int) {
	t.Helper()
	if len(findings) != count {
		t.Fatalf("resource warnings=%v want %d", findings, count)
	}
	for _, finding := range findings {
		if finding.Severity != diagnostic.SeverityWarning || finding.Code != "SDKGEN-W513" || finding.Capability != "resource" || finding.Scope != failure.ScopeCapability || finding.Effect != failure.EffectOmitCapability {
			t.Fatalf("unexpected diagnostic %v", finding)
		}
	}
}

func TestGeneratedResourceOmissionDiagnosticsAndStrictMode(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Resource omissions","version":"1"},"paths":{"/mixed-{id}":{"get":{"operationId":"mixed","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},"/{id}/{id}":{"get":{"operationId":"repeated","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}}
	plan, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 2 {
		t.Fatal(err, diagnostics)
	}
	for _, finding := range diagnostics {
		if finding.Code != "SDKGEN-W513" || finding.Severity != diagnostic.SeverityWarning || finding.Capability != "resource" {
			t.Fatal(finding)
		}
	}
	inspection, err := (Generator{}).InspectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection["GET /mixed-{id}"].ResourceOmission != "unsupported-path-segment" || inspection["GET /{id}/{id}"].ResourceOmission != "duplicate-path-parameter" {
		t.Fatal(inspection)
	}
	testGeneratedModule(t, plan, map[string]string{"resource_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api")
func TestRouteFallbacks(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/same/same"&&r.URL.Path!="/mixed-item"{t.Error(r.URL.Path)};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};if _,present:=reflect.TypeOf(*client).MethodByName("ByID");present{t.Fatal("duplicate path parameter exposed independent selectors")};if _,err:=client.Operations.Repeated(sdk.RepeatedInput{Path:sdk.RepeatedPath{ID:"same"}}).Do(context.Background());err!=nil{t.Fatal(err)};if _,err:=client.Operations.Mixed(sdk.MixedInput{Path:sdk.MixedPath{ID:"item"}}).Do(context.Background());err!=nil{t.Fatal(err)}}
`})
	options.FailOnResourceOmission = true
	_, diagnostics, err = (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 2 {
		t.Fatal(err, diagnostics)
	}
	for _, finding := range diagnostics {
		if finding.Code != "SDKGEN-E513" || finding.Severity != diagnostic.SeverityError {
			t.Fatal(finding)
		}
	}
}

func TestGeneratedPunctuationHTTPMethod(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Native token method","version":"1"},"paths":{"/token":{"additionalOperations":{"!!!":{"operationId":"invoke","responses":{"204":{"description":"OK"}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"method_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api")
func TestNativeTokenMethod(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Method!="!!!"{t.Error(r.Method)};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};if _,err:=client.Operations.Invoke().Do(context.Background());err!=nil{t.Fatal(err)}}
`})
}
