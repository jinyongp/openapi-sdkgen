package output

import (
	"path/filepath"
	"strings"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestClientGenerationRoundTripAndIndependentAssignments(t *testing.T) {
	base := Generation{Generator: "test", Target: "typescript", InputSHA256: strings.Repeat("a", 64), Clients: []ClientGeneration{{Name: "a", Routes: []string{"GET /a"}}, {Name: "b", Routes: []string{"GET /b"}}}}
	output := filepath.Join(t.TempDir(), "sdk")
	if err := PublishArtifacts(output, []generator.Artifact{{Path: "clients/a/index.ts", Data: []byte("export {}")}}, false, &base); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(output)
	if err != nil || !GenerationEqual(manifest.Generation, &base) {
		t.Fatalf("round trip: %#v, %v", manifest, err)
	}
	changed := base
	changed.Clients = []ClientGeneration{{Name: "a", Routes: []string{"GET /b"}}, {Name: "b", Routes: []string{"GET /a"}}}
	if GenerationEqual(&base, &changed) {
		t.Fatal("same union with different assignments matched")
	}
	for _, clients := range [][]ClientGeneration{
		{{Name: "../a", Routes: []string{"GET /a"}}},
		{{Name: "a", Routes: []string{}}},
		{{Name: "a", Routes: []string{"GET /b", "GET /a"}}},
		{{Name: "a", Routes: []string{"GET /a", "GET /a"}}},
		{{Name: "b", Routes: []string{"GET /b"}}, {Name: "a", Routes: []string{"GET /a"}}},
	} {
		changed.Clients = clients
		if err := validateGeneration(changed); err == nil {
			t.Fatalf("invalid identity accepted: %#v", clients)
		}
	}
}
