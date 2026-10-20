package golang

import (
	"fmt"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

// An empty authoring key previously produced an unused sectionBody variable:
// its DTO was encoded but no media encoder consumed it. Reject that contract
// before emission instead of generating a module that cannot compile.
func TestPrepareRejectsEmptyRequestMedia(t *testing.T) {
	for _, required := range []bool{false, true} {
		for _, media := range []string{"", " "} {
			t.Run(fmt.Sprintf("%t/%q", required, media), func(t *testing.T) {
				source := fmt.Sprintf(`{"openapi":"3.0.3","info":{"title":"Test","version":"1"},"paths":{"/items":{"post":{"operationId":"send","requestBody":{"required":%t,"content":{%q:{"schema":{"type":"object"}}}},"responses":{"204":{"description":"Done"}}}}}}`, required, media)
				document, err := compiler.Compile([]byte(source))
				if err != nil {
					t.Fatal(err)
				}
				_, _, err = (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
				if err == nil || !strings.Contains(err.Error(), "non-empty media type or media range") {
					t.Fatal("invalid request media contract accepted", err)
				}
			})
		}
	}
}
