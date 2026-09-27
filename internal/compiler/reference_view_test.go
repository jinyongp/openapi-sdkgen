package sdkgen

import (
	"reflect"
	"strings"
	"testing"
)

func TestOpaqueReferenceEscaperRoundTripsMarkerCollisions(t *testing.T) {
	escaper := newOpaqueReferenceEscaper()
	first := map[string]any{
		"openapi": "3.1.1",
		"x-one": map[string]any{
			"$ref":                            "missing-one.yaml",
			opaqueReferenceMarkerPrefix + "0": "literal preexisting marker-shaped key",
		},
	}
	escapedFirst, changed, err := escaper.escape(first)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("opaque $ref was not escaped")
	}
	firstExtension := escapedFirst.(map[string]any)["x-one"].(map[string]any)
	var firstMarker string
	for key, value := range firstExtension {
		if strings.HasPrefix(key, opaqueReferenceMarkerPrefix) && value == "missing-one.yaml" {
			firstMarker = key
		}
	}
	if firstMarker == "" || firstMarker == opaqueReferenceMarkerPrefix+"0" {
		t.Fatalf("escaped first extension = %#v", firstExtension)
	}

	second := map[string]any{
		"openapi": "3.1.1",
		"x-two": map[string]any{
			"$ref":      "missing-two.yaml",
			firstMarker: "literal marker-shaped key",
		},
	}
	escapedSecond, changed, err := escaper.escape(second)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("second opaque object was not escaped")
	}

	restoredFirst, err := escaper.restore(escapedFirst)
	if err != nil {
		t.Fatal(err)
	}
	restoredSecond, err := escaper.restore(escapedSecond)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restoredFirst, first) {
		t.Fatalf("restored first = %#v, want %#v", restoredFirst, first)
	}
	if !reflect.DeepEqual(restoredSecond, second) {
		t.Fatalf("restored second = %#v, want %#v", restoredSecond, second)
	}
}

func TestOpaqueReferenceEscaperLeavesNamedMapXKeyReferencesActive(t *testing.T) {
	escaper := newOpaqueReferenceEscaper()
	value := map[string]any{
		"components": map[string]any{
			"schemas": map[string]any{
				"Thing": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"x-model": map[string]any{
							"$ref": "#/components/schemas/Base",
						},
					},
				},
				"Base": map[string]any{"type": "string"},
			},
		},
	}
	escaped, changed, err := escaper.escape(value)
	if err != nil {
		t.Fatal(err)
	}
	if changed || !reflect.DeepEqual(escaped, value) {
		t.Fatalf("named-map x-* property was treated as opaque: %#v", escaped)
	}
}

func TestOpaqueReferenceEscaperReturnsOriginalBytesWithoutOpaqueReference(t *testing.T) {
	escaper := newOpaqueReferenceEscaper()
	value := map[string]any{
		"openapi": "3.1.1",
		"paths": map[string]any{
			"/items": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "OK",
						},
					},
				},
			},
		},
	}
	original := []byte("exact-original-bytes")
	got, err := escaper.data(value, original)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("unchanged reference view copied/encoded source: %q", got)
	}
}
