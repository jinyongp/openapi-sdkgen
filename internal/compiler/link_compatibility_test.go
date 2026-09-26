package sdkgen

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestCompatibilityQuarantinesInvalidInlineLinkTargetIdentity(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Link compatibility","version":"1"},
  "paths":{
    "/items":{
      "get":{
        "operationId":"listItems",
        "responses":{
          "200":{
            "description":"OK",
            "links":{
              "validBefore":{"operationId":"getItem"},
              "missingTarget":{},
              "bothTargets":{"operationId":"getItem","operationRef":"#/paths/~1items~1{id}/get"},
              "validAfter":{"operationRef":"#/paths/~1items~1{id}/get"}
            }
          }
        }
      }
    },
    "/items/{id}":{
      "get":{
        "operationId":"getItem",
        "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
        "responses":{"200":{"description":"OK"}}
      }
    }
  }
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 2 || len(result.Document.SemanticRestrictions) != 2 {
		t.Fatalf("compile result = %#v", result)
	}
	links, _ := result.Document.Operations[0].Responses[0].Raw["links"].(map[string]any)
	if len(links) != 2 || links["validBefore"] == nil || links["validAfter"] == nil {
		t.Fatalf("effective links = %#v", links)
	}
	if links["missingTarget"] != nil || links["bothTargets"] != nil {
		t.Fatalf("invalid links survived effective semantics: %#v", links)
	}
	for _, value := range result.Diagnostics {
		if value.Severity != diagnostic.SeverityWarning || value.Code != "SDKGEN-W140" ||
			value.Rule != "COMP-LINK-001" || value.Action != "reject" ||
			value.Scope != failure.ScopeCapability || value.Effect != failure.EffectOmitCapability {
			t.Fatalf("diagnostic = %#v", value)
		}
	}
	for _, restriction := range result.Document.SemanticRestrictions {
		if restriction.RuleID != "COMP-LINK-001" || restriction.Scope != failure.ScopeCapability ||
			restriction.Effect != failure.EffectOmitCapability {
			t.Fatalf("restriction = %#v", restriction)
		}
	}
	metadata := string(result.Document.SourceMetadataJSON)
	for _, name := range []string{"validBefore", "missingTarget", "bothTargets", "validAfter"} {
		if !strings.Contains(metadata, `"`+name+`"`) {
			t.Fatalf("source metadata lost %q: %s", name, metadata)
		}
	}
}

func TestCompatibilityQuarantinesInvalidReusableLinkAtOccurrence(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Reusable Link compatibility","version":"1"},
  "paths":{
    "/items":{
      "get":{
        "operationId":"listItems",
        "responses":{
          "200":{
            "description":"OK",
            "links":{
              "bad":{"$ref":"#/components/links/Bad"},
              "good":{"$ref":"#/components/links/Good"}
            }
          }
        }
      }
    },
    "/items/{id}":{
      "get":{
        "operationId":"getItem",
        "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
        "responses":{"200":{"description":"OK"}}
      }
    }
  },
  "components":{
    "links":{
      "Bad":{},
      "Good":{"operationId":"getItem"}
    }
  }
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 1 || len(result.Document.SemanticRestrictions) != 1 {
		t.Fatalf("compile result = %#v", result)
	}
	links, _ := result.Document.Operations[0].Responses[0].Raw["links"].(map[string]any)
	if len(links) != 1 || links["good"] == nil || links["bad"] != nil {
		t.Fatalf("effective reusable links = %#v", links)
	}
	components, _ := result.Document.Raw["components"].(map[string]any)
	componentLinks, _ := components["links"].(map[string]any)
	if _, exists := componentLinks["Bad"]; !exists {
		t.Fatalf("reusable Link definition was deleted instead of occurrence-quarantined: %#v", componentLinks)
	}
	value := result.Diagnostics[0]
	if value.Location.Pointer != "#/paths/~1items/get/responses/200/links/bad" ||
		value.Rule != "COMP-LINK-001" || value.Scope != failure.ScopeCapability ||
		value.Effect != failure.EffectOmitCapability {
		t.Fatalf("diagnostic = %#v", value)
	}
	if metadata := string(result.Document.SourceMetadataJSON); !strings.Contains(metadata, `"Bad":{}`) ||
		!strings.Contains(metadata, `"#/components/links/Bad"`) {
		t.Fatalf("source metadata lost reusable invalid Link: %s", metadata)
	}
}
