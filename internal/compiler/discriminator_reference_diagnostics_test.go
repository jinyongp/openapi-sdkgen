package sdkgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestCompileResultReportsMissingDiscriminatorMappingReferences(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		for _, reference := range []string{"Missing", "#/components/schemas/Missing", "#%2Fcomponents%2Fschemas%2FMissing"} {
			t.Run(version+"/"+reference, func(t *testing.T) {
				input := []byte(fmt.Sprintf(`{
  "openapi": %q, "info": {"title": "Discriminator references", "version": "1"},
  "paths": {"/jobs": {"get": {"responses": {"200": {
    "description": "Jobs", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Job"}}}
  }}}}},
  "components": {"schemas": {
    "Job": {"oneOf": [{"$ref": "#/components/schemas/Existing"}],
      "discriminator": {"propertyName": "kind", "mapping": {"missing/~variant": %q}}},
    "Existing": {"type": "object", "required": ["kind"], "properties": {"kind": {"type": "string"}}}
  }}
}`, version, reference))
				result, err := CompileResult(input)
				if err != nil {
					t.Fatal(err)
				}
				if result.Document != nil || len(result.Diagnostics) != 1 {
					t.Fatalf("expected one author reference error, got document=%t diagnostics=%#v", result.Document != nil, result.Diagnostics)
				}
				value := result.Diagnostics[0]
				if value.Code != "SDKGEN-E120" || value.Phase != diagnostic.PhaseReferences || value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock ||
					value.Location.Pointer != "#/components/schemas/Job/discriminator/mapping/missing~1~0variant" {
					t.Fatalf("unexpected diagnostic: %#v", value)
				}
			})
		}
	}
}

func TestCompileResultReportsMissingDiscriminatorDefaultMapping(t *testing.T) {
	input := []byte(`{
  "openapi": "3.2.0", "info": {"title": "Default mapping", "version": "1"},
  "paths": {}, "components": {"schemas": {"Job": {
    "oneOf": [{"type": "object"}],
    "discriminator": {"propertyName": "kind", "defaultMapping": "Missing"}
  }}}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "SDKGEN-E120" ||
		result.Diagnostics[0].Location.Pointer != "#/components/schemas/Job/discriminator/defaultMapping" {
		t.Fatalf("expected unresolved default mapping diagnostic, got %#v", result)
	}
}

func TestDiscriminatorReferenceScanPreservesValidAndOpaqueMappings(t *testing.T) {
	for _, reference := range []string{"Target/~", ".Target", "#/components/schemas/Target~1~0", "#%2Fcomponents%2Fschemas%2FTarget~1~0", "#TargetAnchor", "./external.json#/Target", "../external.json#/Target", "/external.json#/Target", "https://example.test/schema.json"} {
		t.Run(reference, func(t *testing.T) {
			var document any
			if err := json.Unmarshal([]byte(fmt.Sprintf(`{
  "openapi": "3.2.0", "info": {"title": "Contexts", "version": "1"}, "paths": {},
  "components": {"schemas": {
    "Job": {"oneOf": [{"type": "object"}], "discriminator": {"propertyName": "kind", "mapping": {"ok": %q}, "defaultMapping": %q},
      "example": {"discriminator": {"mapping": {"bad": "Missing"}}},
      "const": {"discriminator": {"defaultMapping": "Missing"}},
      "x-data": {"discriminator": {"mapping": {"bad": "Missing"}}},
      "properties": {"discriminator": {"type": "object", "properties": {"mapping": {"type": "string"}}}}},
    "Target/~": {"type": "object", "$anchor": "TargetAnchor"},
    ".Target": {"type": "object"}
  }},
  "x-data": {"discriminator": {"mapping": {"bad": "Missing"}}}
}`, reference, reference)), &document); err != nil {
				t.Fatal(err)
			}
			if values := unresolvedLocalReferenceDiagnostics(document, "fixture"); len(values) != 0 {
				t.Fatalf("valid or opaque mapping produced diagnostics: %#v", values)
			}
		})
	}
}

func TestCompileFileReportsInlineDiscriminatorReferenceAndPreservesValidTarget(t *testing.T) {
	for _, target := range []string{"Missing", "Existing"} {
		t.Run(target, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "api.json")
			input := []byte(fmt.Sprintf(`{
  "openapi": "3.0.3", "info": {"title": "Inline mapping", "version": "1"},
  "paths": {"/jobs": {"post": {
    "requestBody": {"content": {"application/json": {"schema": {
      "oneOf": [{"$ref": "#/components/schemas/Existing"}],
      "discriminator": {"propertyName": "kind", "mapping": {"job": %q}}
    }}}}, "responses": {"204": {"description": "Accepted"}}
  }}},
  "components": {"schemas": {"Existing": {"type": "object", "required": ["kind"], "properties": {"kind": {"type": "string"}}}}}
}`, target))
			if err := os.WriteFile(file, input, 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := CompileFileResultWithOptions(file, CompileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if target == "Existing" {
				if result.Document == nil || diagnostic.HasErrors(result.Diagnostics) {
					t.Fatalf("valid mapping blocked: %#v", result.Diagnostics)
				}
				return
			}
			if result.Document != nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "SDKGEN-E120" || result.Diagnostics[0].Location.Source != file ||
				result.Diagnostics[0].Location.Pointer != "#/paths/~1jobs/post/requestBody/content/application~1json/schema/discriminator/mapping/job" {
				t.Fatalf("missing inline mapping reference lost its source identity: %#v", result.Diagnostics)
			}
		})
	}
}
