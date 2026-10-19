package plan

import (
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// SecurityRequirementIDs preserves the established scheme-based identities and
// disambiguates every occupied identity, including names containing the suffix
// delimiter and a declared scheme literally named "anonymous".
func SecurityRequirementIDs(values []ir.SecurityRequirement) []string {
	result := make([]string, 0, len(values))
	used := make(map[string]bool, len(values))
	for _, value := range values {
		names := make([]string, 0, len(value.Schemes))
		for _, scheme := range value.Schemes {
			name := scheme.Name
			if scheme.Reference != "" {
				name = scheme.Reference
			}
			names = append(names, name)
		}
		base := "anonymous"
		if len(names) > 0 {
			base = strings.Join(names, "__")
		}
		id := base
		for suffix := 2; used[id]; suffix++ {
			id = fmt.Sprintf("%s__%d", base, suffix)
		}
		used[id] = true
		result = append(result, id)
	}
	return result
}
