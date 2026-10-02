package typescript

import (
	"bytes"
	"reflect"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestTypeCheckPolicyCoversEveryArtifactAndIsFrozenBeforeEmission(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "selected"}[selected], func(t *testing.T) {
			registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
			options, _ := registry.Resolve([]string{"server", "metadata"})
			options.Clients = map[string]generator.Client{"a": {Selection: &generator.Selection{Operations: []string{"a"}}}, "c": {Selection: &generator.Selection{Operations: []string{"c"}}}}
			if selected {
				options.Selection = &generator.Selection{Routes: []string{"GET /idless/{task-id}"}}
			}
			emit := func(choice *bool) []generator.Artifact {
				t.Helper()
				options.TypeScriptTypeCheck = choice
				// The selection fixture deliberately contains an invalid, excluded
				// extension. Use a valid complete input for both header policies.
				document, err := sdkgen.Compile(bytes.ReplaceAll([]byte(generationSelectionFixture), []byte(`,"x-envelope":false`), nil))
				if err != nil {
					t.Fatal(err)
				}
				plan, diagnostics, err := (Generator{}).Prepare(document, options)
				if err != nil || diagnostic.HasErrors(diagnostics) {
					t.Fatalf("prepare: %v %v", err, diagnostics)
				}
				// Caller-owned configuration cannot mutate the prepared output policy.
				if choice != nil {
					*choice = !*choice
				}
				artifacts, err := (Generator{}).Emit(plan)
				if err != nil {
					t.Fatal(err)
				}
				var streamed []generator.Artifact
				if err := (Generator{}).EmitTo(plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error { streamed = append(streamed, artifact); return nil })); err != nil {
					t.Fatal(err)
				}
				asMap := func(values []generator.Artifact) map[string]string {
					result := map[string]string{}
					for _, artifact := range values {
						result[artifact.Path] = string(artifact.Data)
					}
					return result
				}
				if !reflect.DeepEqual(asMap(artifacts), asMap(streamed)) {
					t.Fatal("Emit and EmitTo differ")
				}
				if err := validateGeneratedArtifacts(artifacts); err != nil {
					t.Fatal(err)
				}
				return artifacts
			}
			defaults := emit(nil)
			unchecked, checked := false, true
			explicit, without := emit(&unchecked), emit(&checked)
			if !reflect.DeepEqual(defaults, explicit) {
				t.Fatal("explicit false changed default bytes")
			}
			if len(defaults) != len(without) {
				t.Fatal("header option changed artifact inventory")
			}
			for i, artifact := range without {
				if bytes.Contains(artifact.Data, []byte("// @ts-nocheck\n")) {
					t.Fatalf("unchecked artifact: %s", artifact.Path)
				}
				if !bytes.HasPrefix(artifact.Data, []byte(generatedFileHeaderFor(false))) {
					t.Fatalf("missing checked header: %s", artifact.Path)
				}
				stripped := bytes.Replace(defaults[i].Data, []byte("// @ts-nocheck\n"), nil, 1)
				if artifact.Path != defaults[i].Path || !bytes.Equal(artifact.Data, stripped) {
					t.Fatalf("policy changed source or execution identity: %s", artifact.Path)
				}
			}
		})
	}
}
