package sdkgen

import "testing"

func TestVersionFeatureRejectCarriesStructuredCompatibilityDiagnostic(t *testing.T) {
	result, err := CompileResult([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Reject","version":"1"},
  "paths":{},
  "webhooks":{"event":{}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || len(result.Diagnostics) != 1 {
		t.Fatalf("result = %#v", result)
	}
	value := result.Diagnostics[0]
	if value.Code != "SDKGEN-E140" || value.Location.Pointer != "#/webhooks" ||
		value.Rule != "COMP-VERSION-003" || value.Action != "reject" {
		t.Fatalf("diagnostic = %#v", value)
	}
}
