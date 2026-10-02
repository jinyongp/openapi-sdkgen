package typescript

import (
	"bytes"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestInspectPlanMatchesGeneratedCallableSurfaces(t *testing.T) {
	document, err := sdkgen.Compile([]byte(strings.Replace(generationSelectionFixture, `"x-envelope":false,`, "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare=%v %#v", err, diagnostics)
	}
	inspection, err := InspectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection["GET /hidden"].Status != "hidden" || inspection["GET /hidden"].Routes {
		t.Fatal("hidden operation exposed")
	}
	idless := inspection["GET /idless/{task-id}"]
	if !idless.Routes || idless.Operations || idless.ResourceCall == nil {
		t.Fatalf("idless=%#v", idless)
	}
	probe := `import {createClient} from "./index.js"; declare const api: ReturnType<typeof createClient>; declare const taskID: string;` + "\n"
	for _, value := range inspection {
		if value.ResourceCall != nil {
			probe += *value.ResourceCall + ";\n"
		}
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	for index := range artifacts {
		artifacts[index].Data = bytes.ReplaceAll(artifacts[index].Data, []byte("// @ts-nocheck\n"), nil)
	}
	compileTypeScriptArtifactSet(t, artifacts, "inspect-probe.ts", probe)
	selected, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Selection: &generator.Selection{Routes: []string{"GET /idless/{task-id}"}}})
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("selected=%v %#v", err, diagnostics)
	}
	narrow, err := InspectPlan(selected)
	if err != nil {
		t.Fatal(err)
	}
	if *narrow["GET /idless/{task-id}"].ResourceCall != *idless.ResourceCall {
		t.Fatal("selected resource path differs from full scope in fixture")
	}
}

func TestInspectPlanReportsActualResourceOmission(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.2.1","info":{"title":"Resource","version":"1"},"paths":{"/files/{name}.json":{"get":{"operationId":"getFile","parameters":[{"name":"name","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},"/users":{"get":{"operationId":"listUsers","responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare=%v %#v", err, diagnostics)
	}
	inspection, err := InspectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	file := inspection["GET /files/{name}.json"]
	if !file.Routes || !file.Operations || file.ResourceCall != nil || file.ResourceOmission != "unsupported-path-segment" {
		t.Fatalf("file=%#v", file)
	}
	users := inspection["GET /users"]
	if users.ResourceCall == nil || !strings.Contains(*users.ResourceCall, "api.users.list(") {
		t.Fatalf("users=%#v", users)
	}
	if _, err := InspectPlan(generator.NewPlan("other", struct{}{})); err == nil {
		t.Fatal("foreign plan accepted")
	}
}
