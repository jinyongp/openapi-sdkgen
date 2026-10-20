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

// A complete binary body has no field/part model. Previously the multipart
// template called an EncodeMedia function that was never generated for it.
func TestPrepareRejectsWholeBinaryMultipart(t *testing.T) {
	addons, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := addons.Resolve([]string{"server"})
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		for _, required := range []bool{false, true} {
			for _, union := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/required=%t/union=%t", version, required, union), func(t *testing.T) {
					binary := `"contentEncoding":"binary"`
					if version == "3.0.3" {
						binary = `"format":"binary"`
					}
					other := ""
					if union {
						other = `,"application/json":{"schema":{"type":"object"}}`
					}
					source := fmt.Sprintf(`{"openapi":%q,"info":{"title":"Test","version":"1"},"paths":{"/items":{"post":{"operationId":"send","requestBody":{"required":%t,"content":{"multipart/form-data":{"schema":{"type":"string",%s}}%s}},"responses":{"204":{"description":"Done"}}}}}}`, version, required, binary, other)
					document, err := compiler.Compile([]byte(source))
					if err != nil {
						t.Fatal(err)
					}
					_, _, err = (Generator{}).Prepare(document, options)
					if err == nil || !strings.Contains(err.Error(), "whole-body binary multipart requests are not supported") || !strings.Contains(err.Error(), "/content/multipart~1form-data/schema") {
						t.Fatal("unsupported binary multipart contract accepted", err)
					}
				})
			}
		}
	}
}
