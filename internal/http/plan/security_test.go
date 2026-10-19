package plan

import (
	"reflect"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestSecurityRequirementIDsAreSelectableIndependently(t *testing.T) {
	values := []ir.SecurityRequirement{}
	for _, name := range []string{"key", "key", "key__2", "anonymous", "", "#/components/securitySchemes/key", "alias"} {
		value := ir.SecurityRequirement{}
		if name != "" {
			value.Schemes = []ir.SecurityRequirementScheme{{Name: name}}
		}
		if name == "alias" {
			value.Schemes[0].Reference = "#/components/securitySchemes/key"
		}
		values = append(values, value)
	}
	want := []string{"key", "key__2", "key__2__2", "anonymous", "anonymous__2", "#/components/securitySchemes/key", "#/components/securitySchemes/key__2"}
	if got := SecurityRequirementIDs(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs %q, want %q", got, want)
	}
}
