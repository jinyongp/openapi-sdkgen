package compatibility

import (
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

func TestNoopPolicyPreservesValueWithoutEvidence(t *testing.T) {
	input := map[string]any{"openapi": "3.1.2", "paths": map[string]any{}}
	result := (NoopPolicy{}).Apply(Context{
		Version: openapidoc.Version31,
		Object:  openapiwalk.ObjectOpenAPI,
		Source:  "openapi.yaml",
		Pointer: "#",
	}, input)
	if len(result.Findings) != 0 || len(result.Ledger) != 0 {
		t.Fatalf("no-op evidence = findings %#v ledger %#v", result.Findings, result.Ledger)
	}
	output, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatalf("no-op result type = %T", result.Value)
	}
	output["x-probe"] = true
	if input["x-probe"] != true {
		t.Fatal("no-op policy copied or replaced the decoded value")
	}
}

func TestCompatibilityClassificationValuesAreStable(t *testing.T) {
	for name, value := range map[string]string{
		"conforming":             string(ConformanceConforming),
		"nonconforming":          string(ConformanceNonconforming),
		"defined":                string(DispositionDefined),
		"ignored":                string(DispositionIgnored),
		"undefined":              string(DispositionUndefined),
		"implementation-defined": string(DispositionImplementationDefined),
		"not-defined":            string(DispositionNotDefined),
		"invalid":                string(DispositionInvalid),
		"preserve":               string(ActionPreserve),
		"ignore":                 string(ActionIgnore),
		"normalize":              string(ActionNormalize),
		"preserve-extension":     string(ActionPreserveExtension),
		"reject":                 string(ActionReject),
	} {
		if value != name {
			t.Fatalf("%s = %q", name, value)
		}
	}
}
