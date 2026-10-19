package output

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestGoGenerationIdentity(t *testing.T) {
	base := Generation{Generator: "test", Target: "golang", InputSHA256: strings.Repeat("a", 64), Golang: &generator.GoOptions{Package: "api", ImportPath: "example.test/api"}}
	if err := validateGeneration(base); err != nil {
		t.Fatal(err)
	}
	for _, options := range []generator.GoOptions{{Package: "custom", ImportPath: "example.test/api"}, {Package: "api", ImportPath: "example.test/another"}} {
		changed := base
		changed.Golang = &options
		if GenerationEqual(&base, &changed) {
			t.Fatalf("Go option drift ignored: %#v", options)
		}
	}
	for _, target := range []string{"typescript", "swift"} {
		mixed := base
		mixed.Target = target
		if err := validateGeneration(mixed); err == nil {
			t.Fatal("accepted Go identity on another target")
		}
	}
	missing := base
	missing.Golang = nil
	if GenerationEqual(&base, &missing) {
		t.Fatal("missing Go identity matched")
	}
	if err := validateGeneration(missing); err == nil {
		t.Fatal("accepted missing Go identity")
	}
	legacy := Generation{Generator: "test", Target: "typescript", InputSHA256: strings.Repeat("a", 64)}
	if err := validateGeneration(legacy); err != nil {
		t.Fatalf("legacy TypeScript fingerprint rejected: %v", err)
	}
}
