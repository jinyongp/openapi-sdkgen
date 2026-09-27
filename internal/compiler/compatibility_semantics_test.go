package sdkgen

import (
	"strings"
	"testing"
)

func TestCompatibilityReferenceObjectSiblingsDoNotOverrideReusableObjects(t *testing.T) {
	tests := []struct {
		name                    string
		version                 string
		parameterSuffix         string
		wantDescription         string
		wantResponseDescription string
	}{
		{
			name:                    "oas30 ignores every sibling",
			version:                 "3.0.3",
			parameterSuffix:         `,"description":"ignored","name":"changed","required":false,"x-vendor":true`,
			wantDescription:         "component",
			wantResponseDescription: "component response",
		},
		{
			name:                    "oas31 preserves description but ignores object fields",
			version:                 "3.1.1",
			parameterSuffix:         `,"description":"reference description","name":"changed","required":false`,
			wantDescription:         "reference description",
			wantResponseDescription: "operation response",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := `{"openapi":"` + test.version + `","info":{"title":"References","version":"1"},"paths":{"/items":{"get":{"operationId":"listItems","parameters":[{"$ref":"#/components/parameters/Limit"` + test.parameterSuffix + `}],"responses":{"200":{"$ref":"#/components/responses/Items","description":"operation response","content":{"application/json":{"schema":{"type":"integer"}}}}}}}},"components":{"parameters":{"Limit":{"name":"limit","in":"query","required":true,"description":"component","schema":{"type":"string"}}},"responses":{"Items":{"description":"component response","content":{"application/json":{"schema":{"type":"string"}}}}}}}`
			result, err := CompileResult([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Diagnostics) != 0 || result.Document == nil {
				t.Fatalf("compile result = %#v", result)
			}
			operation := result.Document.Operations[0]
			if len(operation.Parameters) != 1 {
				t.Fatalf("parameters = %#v", operation.Parameters)
			}
			parameter := operation.Parameters[0]
			if parameter.Name != "limit" || !parameter.Required || parameter.Description != test.wantDescription {
				t.Fatalf("parameter = %#v", parameter)
			}
			response := operation.Responses[0]
			if response.Description != test.wantResponseDescription {
				t.Fatalf("response description = %q, want %q", response.Description, test.wantResponseDescription)
			}
			if len(response.Content) != 1 {
				t.Fatalf("response content = %#v", response.Content)
			}
			schema, _ := response.Content[0].Schema.(map[string]any)
			if schema["type"] != "string" {
				t.Fatalf("response schema = %#v", schema)
			}
			if metadata := string(result.Document.SourceMetadataJSON); !strings.Contains(metadata, `"name":"changed"`) || !strings.Contains(metadata, `"type":"integer"`) {
				t.Fatalf("source metadata lost ignored siblings: %s", metadata)
			}
		})
	}
}

func TestCompatibilityKeepsSchemaContextWhenPropertyNamesCollideWithStructuralTokens(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Schema token collisions","version":"1"},
  "paths":{"/items":{"get":{"operationId":"getItems","responses":{"204":{"description":"OK"}}}}},
  "components":{"schemas":{"Thing":{
    "type":"object",
    "properties":{
      "links":{
        "type":"object",
        "properties":{
          "operationId":{"type":"string"},
          "operationRef":{"type":"string"}
        }
      },
      "properties":{
        "type":"object",
        "nullable":true
      },
      "responses":{"type":"string"},
      "headers":{"type":"string"},
      "content":{"type":"string"},
      "callbacks":{"type":"string"}
    }
  }}}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("compile result = %#v", result)
	}
}

func TestCompatibilityPreservesNativeOpenAPI30BooleanAdditionalProperties(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Boolean additional properties","version":"1"},
  "paths":{},
  "components":{"schemas":{
    "Open":{"type":"object","additionalProperties":true},
    "Closed":{"type":"object","additionalProperties":false}
  }}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("compile result = %#v", result)
	}
	if got := result.Document.ComponentSchemas["Open"]["additionalProperties"]; got != true {
		t.Fatalf("open additionalProperties = %#v", got)
	}
	if got := result.Document.ComponentSchemas["Closed"]["additionalProperties"]; got != false {
		t.Fatalf("closed additionalProperties = %#v", got)
	}
	metadata := string(result.Document.SourceMetadataJSON)
	if !strings.Contains(metadata, `"additionalProperties":true`) ||
		!strings.Contains(metadata, `"additionalProperties":false`) {
		t.Fatalf("source metadata lost boolean additionalProperties: %s", metadata)
	}
}

func TestCompatibilityOAS30SchemaReferenceSiblingsAreIgnoredBeforeVersionValidation(t *testing.T) {
	for _, test := range []struct {
		name            string
		version         string
		wantDescription bool
	}{
		{name: "oas30", version: "3.0.3", wantDescription: false},
		{name: "oas31", version: "3.1.1", wantDescription: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := []byte(`{
  "openapi":"` + test.version + `",
  "info":{"title":"Schema ref siblings","version":"1"},
  "paths":{"/item":{"get":{"operationId":"getItem","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Wrapper"}}}}}}}},
  "components":{"schemas":{
    "Base":{"type":"string"},
    "Wrapper":{"type":"object","properties":{"value":{"$ref":"#/components/schemas/Base","description":"schema reference description"}}}
  }}
}`)
			result, err := CompileResult(input)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Diagnostics) != 0 || result.Document == nil {
				t.Fatalf("compile result = %#v", result)
			}
			wrapper := result.Document.ComponentSchemas["Wrapper"]
			properties, _ := wrapper["properties"].(map[string]any)
			value, _ := properties["value"].(map[string]any)
			_, hasDescription := value["description"]
			if hasDescription != test.wantDescription {
				t.Fatalf("effective schema value = %#v", value)
			}
			if metadata := string(result.Document.SourceMetadataJSON); !strings.Contains(metadata, "schema reference description") {
				t.Fatalf("source metadata lost schema reference sibling: %s", metadata)
			}
		})
	}
}

func TestCompatibilityIgnoresReservedRequestHeadersBeforeNestedReferences(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Reserved headers","version":"1"},
  "paths":{"/items":{"get":{
    "operationId":"listItems",
    "parameters":[
      {"name":"Accept","in":"header","schema":{"$ref":"missing-accept.yaml"}},
      {"name":"content-TYPE","in":"header","schema":{"$ref":"missing-content-type.yaml"}},
      {"$ref":"#/components/parameters/Auth"},
      {"name":"X-Trace","in":"header","schema":{"type":"string"}}
    ],
    "responses":{"204":{"description":"OK"}}
  }}},
  "components":{"parameters":{"Auth":{
    "name":"AUTHORIZATION","in":"header","schema":{"$ref":"missing-auth.yaml"}
  }}}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 || result.Document == nil {
		t.Fatalf("compile result = %#v", result)
	}
	parameters := result.Document.Operations[0].Parameters
	if len(parameters) != 1 || parameters[0].Name != "X-Trace" {
		t.Fatalf("parameters = %#v", parameters)
	}
	metadata := string(result.Document.SourceMetadataJSON)
	for _, value := range []string{"Accept", "content-TYPE", "AUTHORIZATION", "missing-auth.yaml"} {
		if !strings.Contains(metadata, value) {
			t.Fatalf("source metadata missing %q: %s", value, metadata)
		}
	}
}

func TestCompatibilityIgnoresResponseContentTypeAndEncodingNoEffectFields(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Header and encoding ignores","version":"1"},
  "paths":{"/items":{"post":{
    "operationId":"createItem",
    "requestBody":{"content":{
      "application/json":{
        "schema":{"type":"object"},
        "encoding":{"ignored":{"style":"form","headers":{"X-Nested":{"schema":{"$ref":"missing-json.yaml"}}}}}
      },
      "application/x-www-form-urlencoded":{
        "schema":{"type":"object","properties":{"name":{"type":"string"}}},
        "encoding":{"name":{"style":"form","headers":{"X-Nested":{"schema":{"$ref":"missing-form.yaml"}}}}}
      },
      "multipart/form-data":{
        "schema":{"type":"object","properties":{"file":{"type":"string"}}},
        "encoding":{"file":{
          "contentType":"text/plain",
          "style":"form",
          "explode":true,
          "allowReserved":true,
          "headers":{
            "Content-Type":{"schema":{"$ref":"missing-part-content-type.yaml"}},
            "X-Part":{"schema":{"type":"string"}}
          }
        }}
      }
    }},
    "responses":{"200":{
      "description":"OK",
      "headers":{
        "content-TYPE":{"schema":{"$ref":"missing-response.yaml"}},
        "X-Trace":{"schema":{"type":"string"}}
      }
    }}
  }}}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 || result.Document == nil {
		t.Fatalf("compile result = %#v", result)
	}
	operation := result.Document.Operations[0]
	responseHeaders, _ := operation.Responses[0].Raw["headers"].(map[string]any)
	if _, exists := responseHeaders["content-TYPE"]; exists {
		t.Fatalf("response Content-Type survived: %#v", responseHeaders)
	}
	if _, exists := responseHeaders["X-Trace"]; !exists {
		t.Fatalf("control response header missing: %#v", responseHeaders)
	}

	byType := make(map[string]map[string]any)
	for _, media := range operation.RequestBody.Content {
		byType[media.ContentType] = media.Raw
	}
	jsonEncoding, _ := byType["application/json"]["encoding"].(map[string]any)
	if len(jsonEncoding) != 0 {
		t.Fatalf("json encoding = %#v", jsonEncoding)
	}
	formEncoding := byType["application/x-www-form-urlencoded"]["encoding"].(map[string]any)["name"].(map[string]any)
	if _, exists := formEncoding["headers"]; exists || formEncoding["style"] != "form" {
		t.Fatalf("urlencoded encoding = %#v", formEncoding)
	}
	multipartEncoding := byType["multipart/form-data"]["encoding"].(map[string]any)["file"].(map[string]any)
	if multipartEncoding["contentType"] != "text/plain" {
		t.Fatalf("multipart content type = %#v", multipartEncoding)
	}
	for _, field := range []string{"style", "explode", "allowReserved"} {
		if _, exists := multipartEncoding[field]; exists {
			t.Fatalf("multipart field %s survived: %#v", field, multipartEncoding)
		}
	}
	partHeaders, _ := multipartEncoding["headers"].(map[string]any)
	if _, exists := partHeaders["Content-Type"]; exists {
		t.Fatalf("encoding Content-Type survived: %#v", partHeaders)
	}
	if _, exists := partHeaders["X-Part"]; !exists {
		t.Fatalf("encoding control header missing: %#v", partHeaders)
	}
}

func TestCompatibilityRejectsConflictingPathItemReferenceFields(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Path conflict","version":"1"},
  "paths":{
    "/base":{"get":{"operationId":"base","responses":{"204":{"description":"OK"}}}},
    "/alias":{"$ref":"#/paths/~1base","get":{"operationId":"alias","responses":{"204":{"description":"OK"}}}}
  }
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || len(result.Diagnostics) != 1 {
		t.Fatalf("compile result = %#v", result)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Code != "SDKGEN-E120" || diagnostic.Location.Pointer != "#/paths/~1alias/get" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if len(diagnostic.Related) != 1 || diagnostic.Related[0].Pointer != "#/paths/~1base/get" {
		t.Fatalf("related = %#v", diagnostic.Related)
	}
}

func TestCompatibilityPreservesNonConflictingPathItemReferenceSiblings(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Path merge","version":"1"},
  "paths":{
    "/base":{"get":{"operationId":"base","responses":{"204":{"description":"OK"}}}},
    "/alias":{"$ref":"#/paths/~1base","summary":"Alias"}
  }
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 || result.Document == nil {
		t.Fatalf("compile result = %#v", result)
	}
	if len(result.Document.Operations) != 2 {
		t.Fatalf("operations = %#v", result.Document.Operations)
	}
}
