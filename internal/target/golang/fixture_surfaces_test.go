package golang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

// Use the same contracts/addons as TypeScript. Expected public retention comes
// from the prepared TypeScript contract; the compiled Go consumer independently
// verifies that all emitted packages and public definitions form valid Go.
func TestGeneratedGoFixtureSurfaces(t *testing.T) {
	base := filepath.Join("..", "..", "..", "test", "typescript", "fixtures")
	data, err := os.ReadFile(filepath.Join(base, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog modelFixtureCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	for _, fixture := range catalog.Local {
		t.Run(fixture.ID, func(t *testing.T) {
			compiled, err := compiler.CompileFileResultWithOptions(filepath.Join(base, fixture.Input), compiler.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
			if err != nil {
				t.Fatal(err)
			}
			options, err := registry.Resolve(fixture.Addons)
			if err != nil {
				t.Fatal(err)
			}
			options.DiagnosticMode = diagnostic.ModeCollect
			ts, err := generator.PrepareCompilation(typescript.Generator{}, compiled, options)
			if err != nil {
				t.Fatal(err)
			}
			options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
			goPrepared, err := generator.PrepareCompilation(Generator{}, compiled, options)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.ID == "diagnostics" {
				if !diagnostic.HasErrors(ts.Diagnostics) || !diagnostic.HasErrors(goPrepared.Diagnostics) {
					t.Fatal("deliberately invalid fixture stopped failing", ts.Diagnostics, goPrepared.Diagnostics)
				}
				return
			}
			if diagnostic.HasErrors(ts.Diagnostics) || diagnostic.HasErrors(goPrepared.Diagnostics) {
				t.Fatal("fixture planning failed", ts.Diagnostics, goPrepared.Diagnostics)
			}
			tsSurfaces, err := (typescript.Generator{}).InspectPlan(ts.Plan)
			if err != nil {
				t.Fatal(err)
			}
			goSurfaces, err := (Generator{}).InspectPlan(goPrepared.Plan)
			if err != nil {
				t.Fatal(err)
			}
			for route, want := range tsSurfaces {
				got, present := goSurfaces[route]
				if !present {
					t.Fatal("Go lost original route", route)
				}
				if want.Status == "exposed" && got.Status != "exposed" {
					t.Fatal("Go omitted a TypeScript callable operation", route, want, got)
				}
				if want.Status == "hidden" && got.Status != "hidden" {
					t.Fatal("Go exposed a hidden operation", route, want, got)
				}
			}
			testGeneratedModule(t, goPrepared.Plan, map[string]string{})
			if _, err := (typescript.Generator{}).Emit(ts.Plan); err != nil {
				t.Fatal("TypeScript emission", err)
			}
			tsRoutes, err := typescript.EmissionRoutes(ts.Plan)
			if err != nil {
				t.Fatal(err)
			}
			goRoutes, err := EmissionRoutes(goPrepared.Plan)
			if err != nil {
				t.Fatal(err)
			}
			delivered := make(map[string]bool)
			for _, route := range goRoutes {
				delivered[route] = true
			}
			for _, route := range tsRoutes {
				if !delivered[route] {
					t.Fatal("Go did not emit a TS route", route)
				}
			}
			tsInbound, err := typescript.EmissionInbound(ts.Plan)
			if err != nil {
				t.Fatal(err)
			}
			goInbound, err := EmissionInbound(goPrepared.Plan)
			if err != nil {
				t.Fatal(err)
			}
			received := make(map[string]generator.InboundOperationInspection)
			for _, entry := range goInbound {
				received[entry.Identity] = entry
			}
			for _, entry := range tsInbound {
				if got, present := received[entry.Identity]; !present || got != entry {
					t.Fatal("inbound identity differs", entry, got)
				}
			}

			t.Logf("same fixture: TS%d/Go%d declared routes; all generated Go packages compile/run under race", len(tsSurfaces), len(goSurfaces))
		})
	}
}
