package openapi

import "testing"

func TestCollectVersionFeatureErrorsAcrossSupportedVersionLines(t *testing.T) {
	tests := []struct {
		name    string
		version VersionLine
		raw     map[string]any
		want    []string
	}{
		{
			name:    "3.0",
			version: Version30,
			raw: map[string]any{
				"openapi": "3.0.3",
				"info": map[string]any{
					"title":   "Version 3.0",
					"version": "1",
					"summary": "later metadata",
				},
				"paths":    map[string]any{},
				"webhooks": map[string]any{"event": map[string]any{}},
				"components": map[string]any{
					"schemas": map[string]any{
						"Thing": map[string]any{"type": []any{"string", "integer"}},
					},
				},
			},
			want: []string{
				"#/webhooks",
				"#/info/summary",
				"#/components/schemas/Thing/type",
			},
		},
		{
			name:    "3.1",
			version: Version31,
			raw: map[string]any{
				"openapi": "3.1.0",
				"info":    map[string]any{"title": "Version 3.1", "version": "1"},
				"$self":   "https://example.test/openapi.yaml",
				"paths": map[string]any{
					"/items": map[string]any{
						"query": map[string]any{},
					},
				},
				"components": map[string]any{"mediaTypes": map[string]any{}},
			},
			want: []string{
				"#/$self",
				"#/components/mediaTypes",
				"#/paths/~1items/query",
			},
		},
		{
			name:    "3.2",
			version: Version32,
			raw: map[string]any{
				"openapi": "3.2.0",
				"info":    map[string]any{"title": "Version 3.2", "version": "1"},
				"paths":   map[string]any{},
				"components": map[string]any{
					"schemas": map[string]any{
						"A": map[string]any{
							"xml": map[string]any{
								"nodeType":  "attribute",
								"attribute": true,
							},
						},
						"B": map[string]any{
							"xml": map[string]any{
								"nodeType": "element",
								"wrapped":  true,
							},
						},
					},
				},
			},
			want: []string{
				"#/components/schemas/A/xml/attribute",
				"#/components/schemas/B/xml/wrapped",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := CollectVersionFeatureErrors(test.raw, test.version)
			if len(values) != len(test.want) {
				t.Fatalf("findings = %#v, want pointers %#v", values, test.want)
			}
			for index, want := range test.want {
				if values[index].Pointer != want {
					t.Fatalf("finding pointers = %#v, want %#v", values, test.want)
				}
				if values[index].CompatibilityRule() != "COMP-VERSION-003" ||
					values[index].CompatibilityAction() != "reject" {
					t.Fatalf("finding = %#v", values[index])
				}
			}
		})
	}
}
