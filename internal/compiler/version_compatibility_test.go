package sdkgen

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestCompatibilityIgnoresLaterVersionMetadataWithStructuredWarnings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pointer string
		rule    string
		source  string
	}{
		{
			name:    "3.0 info summary",
			input:   `{"openapi":"3.0.3","info":{"title":"Example","version":"1","summary":"later"},"paths":{}}`,
			pointer: "#/info/summary",
			rule:    "COMP-VERSION-001",
			source:  `"summary":"later"`,
		},
		{
			name:    "3.1 server name",
			input:   `{"openapi":"3.1.1","info":{"title":"Example","version":"1"},"servers":[{"url":"/","name":"production"}],"paths":{}}`,
			pointer: "#/servers/0/name",
			rule:    "COMP-VERSION-002",
			source:  `"name":"production"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := CompileResult([]byte(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if result.Document == nil || len(result.Diagnostics) != 1 {
				t.Fatalf("result = %#v", result)
			}
			value := result.Diagnostics[0]
			if value.Severity != diagnostic.SeverityWarning || value.Code != "SDKGEN-W140" ||
				value.Location.Pointer != test.pointer || value.Rule != test.rule || value.Action != "ignore" {
				t.Fatalf("diagnostic = %#v", value)
			}
			if !strings.Contains(string(result.Document.SourceMetadataJSON), test.source) {
				t.Fatalf("source metadata lost %q: %s", test.source, result.Document.SourceMetadataJSON)
			}
		})
	}
}
