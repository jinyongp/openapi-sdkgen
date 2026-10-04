package ir

import (
	"reflect"
	"strings"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

func TestSecuritySchemeLocalResolution(t *testing.T) {
	key := map[string]any{"type": "apiKey", "in": "header", "name": "X-Key"}
	raw := map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Security", "version": "1"},
		"components": map[string]any{"securitySchemes": map[string]any{
			"key": key, "alias": map[string]any{"$ref": "#/components/securitySchemes/key"},
			"slash/name~": key, "literal%2F": key,
			"cycle": map[string]any{"$ref": "#/components/securitySchemes/other"}, "other": map[string]any{"$ref": "#/components/securitySchemes/cycle"},
		}},
		"x-security": map[string]any{"type": "http", "scheme": "bearer"},
	}
	tests := []struct {
		name, lookup, identity, failure string
		allowURI                        bool
	}{
		{"name", "key", "key", "", true},
		{"fragment", "#/components/securitySchemes/key", "key", "", true},
		{"percent fragment", "#%2Fcomponents%2FsecuritySchemes%2F%6Bey", "key", "", true},
		{"pointer escape", "#/components/securitySchemes/slash~1name~0", "slash/name~", "", true},
		{"decode percent once", "#/components/securitySchemes/literal%252F", "literal%2F", "", true},
		{"alias name", "alias", "alias", "", true},
		{"alias fragment", "#/components/securitySchemes/alias", "alias", "", true},
		{"standalone scheme", "#/x-security", "#/x-security", "", true},
		{"missing", "#/components/securitySchemes/missing", "", "unresolved", true},
		{"wrong object", "#/info", "", "Security Scheme Object", true},
		{"external", "./key", "", "external", true},
		{"https", "https://example.invalid/key", "", "external", true},
		{"old name", "key", "key", "", false},
		{"old fragment", "#/components/securitySchemes/key", "", "unknown security scheme", false},
		{"cycle", "cycle", "", "cyclic", true},
		{"malformed percent", "#/components/securitySchemes/%XX", "", "fragment escape", true},
		{"malformed pointer", "#/components/securitySchemes/slash~2name", "", "Pointer escape", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveSecurityScheme(raw, test.lookup, test.allowURI)
			if test.failure != "" {
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("got %v, want %q", err, test.failure)
				}
				return
			}
			if err != nil || got.Name != test.identity {
				t.Fatalf("scheme=%#v err=%v", got, err)
			}
			if got.Type == "apiKey" && got.ParameterName != "X-Key" {
				t.Fatalf("lost alias definition: %#v", got)
			}
		})
	}
	t.Run("component name takes precedence", func(t *testing.T) {
		name := "#/components/securitySchemes/key"
		schemes := raw["components"].(map[string]any)["securitySchemes"].(map[string]any)
		schemes[name] = map[string]any{"type": "apiKey", "in": "header", "name": "X-Precedence"}
		got, err := ResolveSecurityScheme(raw, name, true)
		if err != nil || got.Name != name || got.ParameterName != "X-Precedence" {
			t.Fatalf("scheme=%#v err=%v", got, err)
		}
		delete(schemes, name)
	})
	t.Run("IR inheritance and raw identity", func(t *testing.T) {
		uri := "#/components/securitySchemes/alias"
		security := []any{map[string]any{uri: []any{}}}
		raw["security"] = security
		raw["paths"] = map[string]any{
			"/root": map[string]any{"get": operationFixture("root")},
			"/open": map[string]any{"get": map[string]any{"operationId": "open", "security": []any{}, "responses": map[string]any{"204": map[string]any{"description": "ok"}}}},
		}
		got, err := Build(&openapidoc.Document{Raw: raw})
		if err != nil {
			t.Fatal(err)
		}
		if got.Security[0].Schemes[0].Name != "alias" || got.Security[0].Schemes[0].Reference != uri {
			t.Fatalf("security=%#v", got.Security)
		}
		if !reflect.DeepEqual(got.Security[0].Raw, security[0]) {
			t.Fatal("raw URI metadata changed")
		}
		if got.SecuritySchemes["alias"].Type != "apiKey" || got.SecuritySchemes["alias"].SourceRaw["$ref"] == nil {
			t.Fatal("alias identity or source lost")
		}
		for _, operation := range got.Operations {
			if operation.Path == "/root" && operation.Security[0].Schemes[0].Name != "alias" {
				t.Fatal("root inheritance lost")
			}
			if operation.Path == "/open" && (operation.Security == nil || len(operation.Security) != 0) {
				t.Fatal("empty override lost")
			}
		}
	})
}
