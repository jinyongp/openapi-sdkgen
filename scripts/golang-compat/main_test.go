package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

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
