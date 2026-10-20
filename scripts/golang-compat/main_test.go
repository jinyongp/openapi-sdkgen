package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/golang"
)

func TestVerifiedEmissionRetainsRouteEvidence(t *testing.T) {
	compiled, err := compiler.CompileResult([]byte(`{"openapi":"3.1.0","info":{"title":"Evidence","version":"1"},"paths":{"/items":{"get":{"operationId":"list","responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepareAndEmitVerified(golang.Generator{}, compiled, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/corpus/api"}}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocked || result.Compilation == nil || result.Compilation.Status != "pass" || len(result.Routes) != 1 || result.Routes[0] != "GET /items" || result.ArtifactSHA256 == "" {
		t.Fatal("verified module lost generation evidence", result)
	}
}

func TestCompareRejectsLostHiddenAndUnexplainedOperations(t *testing.T) {
	for _, name := range []string{"missing", "hidden", "unsafe-extra", "native-extra", "unknown-extra", "duplicate-route", "duplicate-inbound", "extra-inbound", "unrelated-fetch-warning"} {
		t.Run(name, func(t *testing.T) {
			route := "GET /items"
			result := report{TypeScript: profile{Routes: []string{route}, Surfaces: map[string]generator.OperationInspection{route: {Status: "exposed"}}}, Go: profile{Routes: []string{route}, Surfaces: map[string]generator.OperationInspection{route: {Status: "exposed"}}}}
			switch name {
			case "missing":
				result.Go.Routes = nil
			case "hidden":
				result.TypeScript.Routes = nil
				result.TypeScript.Surfaces[route] = generator.OperationInspection{Status: "hidden"}
			case "unknown-extra":
				result.Go.Routes = append(result.Go.Routes, "GET /unknown")
			case "duplicate-route":
				result.Go.Routes = append(result.Go.Routes, route)
			case "duplicate-inbound":
				entry := generator.InboundOperationInspection{Identity: "#/webhooks/a/post"}
				result.TypeScript.Inbound = []generator.InboundOperationInspection{entry}
				result.Go.Inbound = []generator.InboundOperationInspection{entry, entry}
			case "extra-inbound":
				result.Go.Inbound = []generator.InboundOperationInspection{{Identity: "#/webhooks/a/post"}}
			case "unsafe-extra", "native-extra", "unrelated-fetch-warning":
				result.TypeScript.Routes = nil
				result.TypeScript.Surfaces[route] = generator.OperationInspection{Status: "omitted"}
				if name == "native-extra" || name == "unrelated-fetch-warning" {
					result.TypeScript.Diagnostics = []diagnostic.Diagnostic{{Code: "SDKGEN-W511", Target: "typescript", Scope: "operation", Route: route, Effect: "omit-operation", Message: "The TypeScript Fetch target cannot issue this request."}}
				}
			}
			if name == "unrelated-fetch-warning" {
				result.TypeScript.Diagnostics[0].Code = "OTHER"
			}
			err := compare(&result)
			if (err == nil) != (name == "native-extra") {
				t.Fatal(name, err)
			}
		})
	}
}
func TestVerifiedInputUsesOriginalPinAndRejectsEscape(t *testing.T) {
	root := t.TempDir()
	data := []byte("original\n")
	if err := os.WriteFile(filepath.Join(root, "input.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d%c", len(data), byte(0))
	hash.Write(data)
	for _, item := range []corpus{{Input: "input.json", SHA256: digest(data), Bytes: int64(len(data))}, {Input: "input.json", GitBlob: hex.EncodeToString(hash.Sum(nil))}} {
		if _, err := verifiedInput(root, item); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []corpus{{Input: "input.json"}, {Input: "../input.json", SHA256: digest(data)}, {Input: "input.json", SHA256: "wrong"}, {Input: "input.json", SHA256: digest(data), Bytes: 1}} {
		if _, err := verifiedInput(root, item); err == nil {
			t.Fatal("invalid pin accepted", item)
		}
	}
}

func TestVerifyGeneratedModuleChecksActualCompilerResult(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "example.go")
	if err := os.WriteFile(path, []byte("package example\nfunc invalid( {\n"), 0600); err != nil {
		t.Fatal(err)
	}
	failed := verifyModule("golang", directory, "")
	if failed.Status != "fail" || failed.Detail == "" {
		t.Fatal("invalid module accepted", failed)
	}
	if err := os.WriteFile(path, []byte("package example\nfunc Value() int {return 42}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	passed := verifyModule("golang", directory, "")
	if passed.Status != "pass" {
		t.Fatal("valid module rejected", passed)
	}
}

func TestVerifyGeneratedModuleRequiresCompletedHTTPContract(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	fixtures := filepath.Join(root, "test", "compatibility", "conformance")
	if err := os.MkdirAll(fixtures, 0755); err != nil {
		t.Fatal(err)
	}
	module := t.TempDir()
	if err := os.WriteFile(filepath.Join(module, "value.go"), []byte("package corpus\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFLAGS", "-run=^$")
	for _, test := range []struct{ body, status string }{{"", "pass"}, {`t.Skip("not executed")`, "fail"}, {`t.Fatal("wrong request")`, "fail"}} {
		data := []byte("package corpus_test\nimport \"testing\"\nfunc TestProbeHTTPContract(t *testing.T){" + test.body + "}\n")
		if err := os.WriteFile(filepath.Join(fixtures, "probe_http_test.go.txt"), data, 0600); err != nil {
			t.Fatal(err)
		}
		result := verifyModule("golang", module, "", "probe_http_test.go.txt")
		if result.Status != test.status || test.status == "pass" && (len(result.ContractTests) != 1 || result.ContractTests[0] != "TestProbeHTTPContract") {
			t.Fatal(test.body, result)
		}
	}
	if result := verifyModule("golang", module, "", "../probe_http_test.go.txt"); result.Status != "fail" {
		t.Fatal("escaping fixture accepted")
	}
}
