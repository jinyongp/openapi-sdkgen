package golang

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func TestUnusedSchemaLiteralCanBeLinkedAway(t *testing.T) {
	document, _ := healthPlan(t)
	const marker = "sdkgen_unused_literal_6de638c6fe72427b9a664b665d49738a"
	document.Operations = append(document.Operations, ir.Operation{RouteKey: "GET /unused", OperationID: "getUnused", Method: "GET", Path: "/unused", Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: map[string]any{"const": marker}}}}}})
	document.Operations = append(document.Operations, ir.Operation{RouteKey: "GET /unused-url", OperationID: "getUnusedURL", Method: "GET", Path: "/unused-url", Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: map[string]any{"format": "uri", "$vocabulary": map[string]any{schemaplan.FormatAssertionVocabulary: true}}}}}}})
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	files := map[string]string{"go.mod": "module example.test/consumer\n\ngo 1.27.0\n"}
	for _, artifact := range artifacts {
		files[filepath.Join("api", artifact.Path)] = string(artifact.Data)
	}
	for _, operation := range []string{"GetHealth", "GetUnused", "GetUnusedURL"} {
		files[strings.ToLower(operation)+"/main.go"] = `package main
import("context";"fmt";sdk "example.test/consumer/api")
func main(){client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test"});if err!=nil{panic(err)};fmt.Println(client.Operations.` + operation + `().Do(context.Background()))}
`
	}
	for name, data := range files {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, operation := range []string{"GetHealth", "GetUnused", "GetUnusedURL"} {
		binary := filepath.Join(root, operation+".bin")
		command := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-buildvcs=false", "-o", binary, "./"+strings.ToLower(operation))
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOMODCACHE="+t.TempDir())
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build consumer: %v %s", err, output)
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if retained := bytes.Contains(data, []byte(marker)); retained != (operation == "GetUnused") {
			t.Errorf("%s: unused schema literal retained=%t", operation, retained)
		}
		if retained := bytes.Contains(data, []byte("https://format.invalid/")); retained != (operation == "GetUnusedURL") {
			t.Errorf("%s: optional URL initializer retained=%t", operation, retained)
		}
	}
}
