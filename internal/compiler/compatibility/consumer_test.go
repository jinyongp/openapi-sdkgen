package compatibility

import (
	"reflect"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

func TestConsumerPolicyFiltersReferenceObjectSiblingsByVersion(t *testing.T) {
	policy := ConsumerPolicy{}
	input := map[string]any{
		"$ref":        "#/components/parameters/Base",
		"summary":     "summary",
		"description": "description",
		"name":        "override",
		"x-vendor":    true,
	}
	tests := []struct {
		name    string
		version openapidoc.VersionLine
		want    map[string]any
		rule    string
	}{
		{
			name:    "3.0",
			version: openapidoc.Version30,
			want:    map[string]any{"$ref": "#/components/parameters/Base"},
			rule:    RuleReference30Siblings,
		},
		{
			name:    "3.1",
			version: openapidoc.Version31,
			want: map[string]any{
				"$ref":        "#/components/parameters/Base",
				"summary":     "summary",
				"description": "description",
			},
			rule: RuleReference31Fields,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := policy.Apply(Context{
				Version: test.version,
				Object:  openapiwalk.ObjectParameter,
				Source:  "openapi.yaml",
				Pointer: "#/paths/~1items/get/parameters/0",
			}, input)
			if !result.Changed || result.Omit || !reflect.DeepEqual(result.Value, test.want) {
				t.Fatalf("result = %#v", result)
			}
			if len(result.Ledger) != 1 || result.Ledger[0].RuleID != test.rule || result.Ledger[0].Action != ActionIgnore {
				t.Fatalf("ledger = %#v", result.Ledger)
			}
		})
	}
}

func TestConsumerPolicyIgnoresReservedHeaderParameters(t *testing.T) {
	policy := ConsumerPolicy{}
	for _, name := range []string{"Accept", "content-type", "AUTHORIZATION"} {
		result := policy.Apply(Context{
			Version: openapidoc.Version32,
			Object:  openapiwalk.ObjectParameter,
			Pointer: "#/paths/~1items/get/parameters/0",
		}, map[string]any{"name": name, "in": "header", "schema": map[string]any{"$ref": "missing.yaml"}})
		if !result.Omit || len(result.Ledger) != 1 || result.Ledger[0].RuleID != RuleReservedHeader {
			t.Fatalf("%s result = %#v", name, result)
		}
	}
	control := policy.Apply(Context{
		Version: openapidoc.Version32,
		Object:  openapiwalk.ObjectParameter,
		Pointer: "#/paths/~1items/get/parameters/0",
	}, map[string]any{"name": "X-Trace", "in": "header"})
	if control.Omit || control.Changed {
		t.Fatalf("control = %#v", control)
	}
}

func TestConsumerPolicyDistinguishesResponseAndEncodingHeaders(t *testing.T) {
	policy := ConsumerPolicy{}
	tests := []struct {
		pointer string
		rule    string
	}{
		{"#/paths/~1items/get/responses/200/headers/content-type", RuleResponseContentType},
		{"#/paths/~1items/post/requestBody/content/multipart~1form-data/encoding/file/headers/Content-Type", RuleEncodingHeaders},
	}
	for _, test := range tests {
		result := policy.Apply(Context{
			Version: openapidoc.Version31,
			Object:  openapiwalk.ObjectHeader,
			Pointer: test.pointer,
		}, map[string]any{"schema": map[string]any{"type": "string"}})
		if !result.Omit || len(result.Ledger) != 1 || result.Ledger[0].RuleID != test.rule {
			t.Fatalf("%s result = %#v", test.pointer, result)
		}
	}
	component := policy.Apply(Context{
		Version: openapidoc.Version31,
		Object:  openapiwalk.ObjectHeader,
		Pointer: "#/components/headers/Content-Type",
	}, map[string]any{"schema": map[string]any{"type": "string"}})
	if component.Omit {
		t.Fatalf("component header omitted = %#v", component)
	}
}

func TestConsumerPolicyAppliesEncodingFieldApplicability(t *testing.T) {
	policy := ConsumerPolicy{}
	jsonResult := policy.Apply(Context{
		Version: openapidoc.Version31,
		Object:  openapiwalk.ObjectEncoding,
		Pointer: "#/paths/~1items/post/requestBody/content/application~1json/encoding/value",
	}, map[string]any{"style": "form"})
	if !jsonResult.Omit {
		t.Fatalf("json encoding = %#v", jsonResult)
	}

	urlencoded := policy.Apply(Context{
		Version: openapidoc.Version31,
		Object:  openapiwalk.ObjectEncoding,
		Pointer: "#/paths/~1items/post/requestBody/content/application~1x-www-form-urlencoded/encoding/value",
	}, map[string]any{"headers": map[string]any{"x-part": map[string]any{}}, "style": "form"})
	if got := urlencoded.Value.(map[string]any); !urlencoded.Changed || got["headers"] != nil || got["style"] != "form" {
		t.Fatalf("urlencoded = %#v", urlencoded)
	}

	multipart30 := policy.Apply(Context{
		Version: openapidoc.Version30,
		Object:  openapiwalk.ObjectEncoding,
		Pointer: "#/paths/~1items/post/requestBody/content/multipart~1form-data/encoding/file",
	}, map[string]any{"contentType": "text/plain", "style": "form", "explode": true, "allowReserved": true})
	got := multipart30.Value.(map[string]any)
	if !multipart30.Changed || got["contentType"] != "text/plain" || got["style"] != nil || got["explode"] != nil || got["allowReserved"] != nil {
		t.Fatalf("multipart 3.0 = %#v", multipart30)
	}
}
