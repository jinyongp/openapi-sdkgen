package sdkgen

import (
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestFailFastCompilerStopBoundaryBaseline(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		code          string
		skippedPhases []diagnostic.Phase
	}{
		{
			name:  "compatibility",
			input: `{"openapi":"3.0.3","info":{"title":"Compatibility","version":"1"},"paths":{},"components":{"schemas":{"Value":{"type":["string","integer"]}}}}`,
			code:  "SDKGEN-E140",
			skippedPhases: []diagnostic.Phase{
				diagnostic.PhaseReferences,
				diagnostic.PhaseNormalize,
				diagnostic.PhaseIR,
			},
		},
		{
			name:  "reserved extension",
			input: `{"openapi":"3.1.0","info":{"title":"Reserved","version":"1"},"x-sdkgen-owned":true,"paths":{}}`,
			code:  "SDKGEN-E160",
			skippedPhases: []diagnostic.Phase{
				diagnostic.PhaseReferences,
				diagnostic.PhaseNormalize,
				diagnostic.PhaseOpenAPI,
				diagnostic.PhaseIR,
			},
		},
		{
			name:  "references",
			input: `{"openapi":"3.1.0","info":{"title":"References","version":"1"},"paths":{},"components":{"schemas":{"Value":{"$ref":"#/components/schemas/Missing"}}}}`,
			code:  "SDKGEN-E120",
			skippedPhases: []diagnostic.Phase{
				diagnostic.PhaseNormalize,
				diagnostic.PhaseOpenAPI,
				diagnostic.PhaseIR,
			},
		},
		{
			name:  "openapi version",
			input: `{"openapi":"3.0.3","info":{"title":"Version","version":"1"},"paths":{},"webhooks":{}}`,
			code:  "SDKGEN-E140",
			skippedPhases: []diagnostic.Phase{
				diagnostic.PhaseIR,
			},
		},
		{
			name:          "ir",
			input:         `{"openapi":"3.1.0","info":{"title":"IR","version":"1"},"paths":[]}`,
			code:          "SDKGEN-E150",
			skippedPhases: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := CompileResult([]byte(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if result.Document != nil || len(result.Diagnostics) != 1 {
				t.Fatalf("result = %#v", result)
			}
			if result.Diagnostics[0].Code != test.code {
				t.Fatalf("diagnostic = %#v, want code %s", result.Diagnostics[0], test.code)
			}
			if len(result.SkippedPhases) != len(test.skippedPhases) {
				t.Fatalf("skipped phases = %#v, want %#v", result.SkippedPhases, test.skippedPhases)
			}
			for index, phase := range test.skippedPhases {
				if result.SkippedPhases[index].Phase != phase {
					t.Fatalf("skipped phases = %#v, want %#v", result.SkippedPhases, test.skippedPhases)
				}
			}
		})
	}
}
