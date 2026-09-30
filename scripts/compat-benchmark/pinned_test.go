package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPinnedLocalCorpusChecksHashesVersionAndClosure(t *testing.T) {
	root := t.TempDir()
	data := []byte(`{"openapi":"3.2.0","info":{"title":"Pinned","version":"1"},"paths":{}}`)
	digest := sha256.Sum256(data)
	spec := corpusSpec{ID: "normative", Cohort: "normative", Input: "root.json", SHA256: hex.EncodeToString(digest[:]), OpenAPIVersion: "3.2.0", EvidenceKind: "normative", SourceURL: "https://spec.openapis.org/oas/v3.2.0.html", Revision: "fixture-1", Trust: "local-only"}
	manifest := benchmarkManifest{SchemaVersion: 1, Pinned: true, Corpora: []corpusSpec{spec}, Files: []pinnedCorpusFile{{Input: "closure.json", SHA256: spec.SHA256}}}
	if err := os.WriteFile(filepath.Join(root, "root.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "closure.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := verifyMaterializedCorpus(root, manifest, "local"); err != nil {
		t.Fatal(err)
	}
	manifest.Corpora[0].OpenAPIVersion = "3.1.2"
	if err := verifyMaterializedCorpus(root, manifest, "local"); err == nil {
		t.Fatal("wrong version accepted")
	}
	manifest.Corpora[0] = spec
	if err := os.WriteFile(filepath.Join(root, "closure.json"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyMaterializedCorpus(root, manifest, "local"); err == nil {
		t.Fatal("changed closure accepted")
	}
	manifest.Corpora[0].SHA256 = ""
	if err := validateManifest(manifest); err == nil {
		t.Fatal("unpinned member accepted")
	}
}

func TestOpenAPI32FeatureDetectorsUseSemanticLocations(t *testing.T) {
	data, err := os.ReadFile("../../test/fixtures/support-gaps/oas32-normative.json")
	if err != nil {
		t.Fatal(err)
	}
	value, err := decodeFeatureInput(data)
	if err != nil {
		t.Fatal(err)
	}
	features, version := detectFeatures(value)
	if version != "3.2.0" {
		t.Fatal(version)
	}
	for _, name := range []string{"operation.query", "parameter.querystring", "document.media-types", "media.item-schema", "media.jsonl", "media.json-seq", "media.multipart-positional-encoding", "schema.discriminator-default-mapping", "security.oauth2-metadata", "security.device-authorization", "security.deprecated", "schema.xml-node-type"} {
		if !slices.Contains(features, name) {
			t.Errorf("missing %s: %v", name, features)
		}
	}
	opaque := map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Opaque", "version": "1"}, "paths": map[string]any{}, "components": map[string]any{"schemas": map[string]any{"Payload": map[string]any{"type": "object", "example": value}}}}
	features, _ = detectFeatures(opaque)
	for _, name := range features {
		if name != "oas.version.3.2" {
			t.Errorf("literal example counted as feature %s", name)
		}
	}
}
