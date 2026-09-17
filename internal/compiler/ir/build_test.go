package ir

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

func TestBuildExtractsOperationsDeterministically(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info": map[string]any{
			"title":   "Example",
			"version": "0.1.0",
		},
		"servers": []any{map[string]any{"url": "/v1"}},
		"paths": map[string]any{
			"/items/{itemId}": map[string]any{
				"get":   operationFixture("getItem"),
				"query": operationFixture("queryItem"),
				"additionalOperations": map[string]any{
					"PURGE": operationFixture("purgeItem"),
				},
			},
			"/alpha": map[string]any{
				"get": operationFixture("getAlpha"),
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"Item": map[string]any{"type": "object"},
			},
		},
	}}

	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]string, 0, len(model.Operations))
	for _, operation := range model.Operations {
		gotIDs = append(gotIDs, operation.OperationID)
		if operation.Parameters == nil {
			t.Fatalf("compiled operation %q has no normalized parameter slice", operation.OperationID)
		}
		if operation.Responses == nil {
			t.Fatalf("compiled operation %q has no normalized response slice", operation.OperationID)
		}
	}
	wantIDs := []string{"getAlpha", "getItem", "purgeItem", "queryItem"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("operation order = %v, want %v", gotIDs, wantIDs)
	}
	if got := model.Operations[1].PathParameterOrder; !reflect.DeepEqual(got, []string{"itemId"}) {
		t.Fatalf("path parameters = %v", got)
	}
	getItem := model.Operations[1]
	if getItem.Pointer != "#/paths/~1items~1{itemId}/get" ||
		!getItem.Extensions.Envelope.Present || !getItem.Extensions.Envelope.Valid || getItem.Extensions.Envelope.Value != "data" ||
		!getItem.Extensions.Pagination.Present || getItem.Extensions.Pagination.Pointer != "#/paths/~1items~1{itemId}/get/x-pagination" ||
		!getItem.Extensions.Visibility.Present || !getItem.Extensions.Visibility.Valid || getItem.Extensions.Visibility.Value != "public" {
		t.Fatalf("typed operation extensions = %#v", getItem)
	}
	if _, ok := model.ComponentSchemas["Item"]; !ok {
		t.Fatal("component schema not preserved")
	}
}

func TestBuildNormalizesOperationRequestContracts(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Requests", "version": "1"},
		"paths": map[string]any{
			"/items/{itemId}": map[string]any{
				"parameters": []any{
					map[string]any{"name": "itemId", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
					map[string]any{"$ref": "#/components/parameters/Trace~1Header"},
					map[string]any{"name": "limit", "in": "query", "required": true, "schema": map[string]any{"type": "integer"}},
				},
				"post": map[string]any{
					"operationId": "createItem",
					"parameters": []any{
						map[string]any{"name": "limit", "in": "query", "required": false, "style": "form", "explode": false, "allowReserved": true, "schema": map[string]any{"type": "number"}},
					},
					"requestBody": map[string]any{"$ref": "#/components/requestBodies/Payload", "description": "Operation payload", "required": true},
					"responses":   map[string]any{"204": map[string]any{"description": "OK"}},
				},
			},
		},
		"components": map[string]any{
			"parameters": map[string]any{
				"Trace/Header": map[string]any{"name": "x-trace", "in": "header", "description": "Trace ID", "schema": map[string]any{"type": "string"}},
			},
			"requestBodies": map[string]any{
				"Payload": map[string]any{"content": map[string]any{
					"application/json": map[string]any{"$ref": "#/components/mediaTypes/Payload"},
					"text/plain":       map[string]any{"schema": map[string]any{"type": "string"}},
				}},
			},
			"mediaTypes": map[string]any{
				"Payload": map[string]any{"schema": map[string]any{"type": "object", "required": []any{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}}},
			},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 1 {
		t.Fatalf("operations = %#v", model.Operations)
	}
	operation := model.Operations[0]
	if len(operation.Parameters) != 3 {
		t.Fatalf("parameters = %#v", operation.Parameters)
	}
	if got := operation.Parameters[0]; got.Name != "itemId" || got.Location != "path" || got.Style != "simple" || got.Explode || !got.Required {
		t.Fatalf("path parameter = %#v", got)
	}
	if got := operation.Parameters[1]; got.Name != "x-trace" || got.Pointer != "#/components/parameters/Trace~1Header" || got.Style != "simple" || got.Explode || got.Description != "Trace ID" {
		t.Fatalf("referenced parameter = %#v", got)
	}
	if got := operation.Parameters[2]; got.Name != "limit" || got.Required || got.Style != "form" || got.Explode || !got.AllowReserved || got.Schema.(map[string]any)["type"] != "number" {
		t.Fatalf("operation override parameter = %#v", got)
	}
	if operation.RequestBody == nil || !operation.RequestBody.Required || operation.RequestBody.Description != "Operation payload" || operation.RequestBody.Pointer != "#/components/requestBodies/Payload" {
		t.Fatalf("request body = %#v", operation.RequestBody)
	}
	if got := operation.RequestBody.Content; len(got) != 2 || got[0].ContentType != "application/json" || got[1].ContentType != "text/plain" {
		t.Fatalf("request media = %#v", got)
	}
	if schema, ok := operation.RequestBody.Content[0].Schema.(map[string]any); !ok || schema["type"] != "object" {
		t.Fatalf("resolved media schema = %#v", operation.RequestBody.Content[0].Schema)
	}
}

func TestBuildNormalizesOperationResponses(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Responses", "version": "1"},
		"paths": map[string]any{
			"/items": map[string]any{
				"get": map[string]any{
					"operationId": "listItems",
					"responses": map[string]any{
						"200": map[string]any{"$ref": "#/components/responses/Items", "description": "Operation items"},
						"204": map[string]any{"description": "No content"},
					},
				},
			},
		},
		"components": map[string]any{
			"responses": map[string]any{
				"Items": map[string]any{
					"description": "Component items",
					"summary":     "Items summary",
					"content": map[string]any{
						"application/json": map[string]any{"$ref": "#/components/mediaTypes/Items"},
					},
				},
			},
			"mediaTypes": map[string]any{
				"Items": map[string]any{"schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
			},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	operation := model.Operations[0]
	if len(operation.Responses) != 2 {
		t.Fatalf("responses = %#v", operation.Responses)
	}
	response := operation.Responses[0]
	if response.Status != "200" || response.Description != "Operation items" || response.Summary != "Items summary" || response.Pointer != "#/paths/~1items/get/responses/200" {
		t.Fatalf("normalized response = %#v", response)
	}
	if response.SourceRaw["$ref"] != "#/components/responses/Items" || len(response.Content) != 1 || response.Content[0].ContentType != "application/json" {
		t.Fatalf("response source/media = %#v", response)
	}
	if schema, ok := response.Content[0].Schema.(map[string]any); !ok || schema["type"] != "array" {
		t.Fatalf("resolved response media schema = %#v", response.Content[0].Schema)
	}
	if got := operation.Responses[1]; got.Status != "204" || got.Description != "No content" || len(got.Content) != 0 {
		t.Fatalf("no-content response = %#v", got)
	}
}

func TestBuildNormalizesSecurityAndEffectiveServers(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Security", "version": "1"},
		"servers": []any{map[string]any{
			"url": "https://{region}.example.test/v1",
			"variables": map[string]any{"region": map[string]any{
				"default": "kr", "enum": []any{"kr", "us"}, "description": "Region",
			}},
		}},
		"security": []any{map[string]any{"Bearer": []any{}}},
		"paths": map[string]any{
			"/root": map[string]any{"get": map[string]any{"operationId": "root", "responses": map[string]any{"204": map[string]any{"description": "OK"}}}},
			"/path": map[string]any{
				"servers": []any{map[string]any{"url": "/v2"}},
				"get":     map[string]any{"operationId": "path", "responses": map[string]any{"204": map[string]any{"description": "OK"}}},
			},
			"/operation": map[string]any{
				"servers": []any{map[string]any{"url": "/ignored"}},
				"get": map[string]any{
					"operationId": "operation",
					"servers":     []any{map[string]any{"url": "/v3"}},
					"security":    []any{},
					"responses":   map[string]any{"204": map[string]any{"description": "OK"}},
				},
			},
		},
		"components": map[string]any{"securitySchemes": map[string]any{
			"Bearer": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
		}},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]Operation, len(model.Operations))
	for _, operation := range model.Operations {
		byPath[operation.Path] = operation
	}
	root := byPath["/root"]
	if len(root.Servers) != 1 || root.Servers[0].Pointer != "#/servers/0" || root.Servers[0].Variables[0].Default != "kr" {
		t.Fatalf("root effective servers = %#v", root.Servers)
	}
	if root.SecurityDeclared || len(root.Security) != 1 || len(root.Security[0].Schemes) != 1 || root.Security[0].Schemes[0].Name != "Bearer" {
		t.Fatalf("root inherited security = %#v declared=%t", root.Security, root.SecurityDeclared)
	}
	if got := byPath["/path"].Servers; len(got) != 1 || got[0].Pointer != "#/paths/~1path/servers/0" || got[0].URL != "/v2" {
		t.Fatalf("path effective servers = %#v", got)
	}
	op := byPath["/operation"]
	if len(op.Servers) != 1 || op.Servers[0].Pointer != "#/paths/~1operation/get/servers/0" || op.Servers[0].URL != "/v3" {
		t.Fatalf("operation effective servers = %#v", op.Servers)
	}
	if !op.SecurityDeclared || op.Security == nil || len(op.Security) != 0 {
		t.Fatalf("operation security override = %#v declared=%t", op.Security, op.SecurityDeclared)
	}
	if scheme := model.SecuritySchemes["Bearer"]; scheme.Type != "http" || scheme.Scheme != "bearer" || scheme.BearerFormat != "JWT" {
		t.Fatalf("security scheme = %#v", scheme)
	}
}

func TestBuildPreservesOpenAPI32SurfaceAndExtractsNewMethods(t *testing.T) {
	data, err := os.ReadFile("../openapi/testdata/oas32-conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := openapidoc.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Build(source)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(model.Raw, source.Raw) {
		t.Fatal("IR root raw document differs from parsed source")
	}
	if got, want := model.OpenAPIVersion, "3.2.0"; got != want {
		t.Fatalf("OpenAPI version = %q, want %q", got, want)
	}
	if got, want := model.OpenAPIVersionLine, "3.2"; got != want {
		t.Fatalf("OpenAPI version line = %q, want %q", got, want)
	}
	if len(model.Servers) != 1 || model.Servers[0].URL != "https://{region}.example.test/v1" || model.Servers[0].Description != "Regional API" || model.Servers[0].Pointer != "#/servers/0" {
		t.Fatalf("servers = %#v", model.Servers)
	}
	if got := model.Servers[0].Variables; len(got) != 1 || got[0].Name != "region" || got[0].Default == "" {
		t.Fatalf("server variables = %#v", got)
	}

	methods := make(map[string]Operation, len(model.Operations))
	for _, operation := range model.Operations {
		methods[operation.Method] = operation
	}
	for method, operationID := range map[string]string{
		"GET":      "getItem",
		"QUERY":    "queryItem",
		"PURGE":    "purgeItem",
		"m-SEARCH": "searchItemByCustomMethod",
	} {
		operation, ok := methods[method]
		if !ok {
			t.Errorf("%s operation was not extracted", method)
			continue
		}
		if operation.OperationID != operationID {
			t.Errorf("%s operation ID = %q, want %q", method, operation.OperationID, operationID)
		}
	}

	get := methods["GET"]
	pathFromRoot := source.Raw["paths"].(map[string]any)["/items/{itemId}"].(map[string]any)
	if !reflect.DeepEqual(get.PathItemRaw, pathFromRoot) {
		t.Fatal("path-level summary, description, parameters, servers, or extensions were not preserved")
	}
	operationFromRoot := pathFromRoot["get"].(map[string]any)
	if !reflect.DeepEqual(get.Raw, operationFromRoot) {
		t.Fatal("operation callbacks, security, externalDocs, or extensions were not preserved")
	}

	for _, field := range []string{"webhooks", "components", "security", "servers", "tags", "externalDocs", "x-root-extension"} {
		if !reflect.DeepEqual(model.Raw[field], source.Raw[field]) {
			t.Errorf("root field %q was not preserved", field)
		}
	}
	item := model.ComponentSchemas["Item"]
	if _, ok := item["exampleVocabularyKeyword"]; !ok {
		t.Error("unknown JSON Schema vocabulary keyword was not preserved")
	}
	rootExtension := model.Raw["x-root-extension"].(map[string]any)
	nested := rootExtension["nested"].(map[string]any)
	if got := nested["count"]; got != json.Number("9007199254740993") {
		t.Fatalf("extension number = %#v", got)
	}
}

func TestBuildPreservesSupportedVersionProvenance(t *testing.T) {
	for version, wantLine := range map[string]string{
		"3.0.3": "3.0",
		"3.1.1": "3.1",
		"3.2.0": "3.2",
	} {
		t.Run(version, func(t *testing.T) {
			schema := `{"type":"object"}`
			if wantLine != "3.0" {
				schema = `{"$id":"https://example.test/schemas/dynamic","$dynamicRef":"#node"}`
			}
			source, err := openapidoc.Read([]byte(`{
  "openapi": "` + version + `",
  "info": {"title": "Versioned", "version": "1"},
  "paths": {},
  "components": {"schemas": {"Dynamic": ` + schema + `}}
}`))
			if err != nil {
				t.Fatal(err)
			}
			model, err := Build(source)
			if err != nil {
				t.Fatal(err)
			}
			if model.OpenAPIVersion != version || model.OpenAPIVersionLine != wantLine {
				t.Fatalf("version provenance = %q (%q), want %q (%q)", model.OpenAPIVersion, model.OpenAPIVersionLine, version, wantLine)
			}
			if wantLine != "3.0" && model.ComponentSchemas["Dynamic"]["$dynamicRef"] != "#node" {
				got := model.ComponentSchemas["Dynamic"]["$dynamicRef"]
				t.Fatalf("schema reference keyword = %#v", got)
			}
		})
	}
}

func TestBuildAcceptsWebhookOnlyOpenAPI32Document(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info": map[string]any{
			"title":   "Webhook-only API",
			"version": "1.0.0",
		},
		"webhooks": map[string]any{
			"event": map[string]any{
				"post": operationFixture("eventWebhook"),
			},
		},
	}}

	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 0 {
		t.Fatalf("path operations = %d, want 0", len(model.Operations))
	}
	if !reflect.DeepEqual(model.Raw["webhooks"], document.Raw["webhooks"]) {
		t.Fatal("webhooks were not preserved")
	}
}

func TestBuildRejectsNonObjectPaths(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Invalid", "version": "1.0.0"},
		"paths":   []any{},
	}}
	if _, err := Build(document); err == nil {
		t.Fatal("non-object paths accepted")
	}
}

func TestBuildSkipsPathExtensionsAndRejectsOpenAPI32MethodsInEarlierLines(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1"} {
		t.Run(version, func(t *testing.T) {
			document := &openapidoc.Document{Version: openapidoc.VersionLine(version[:3]), Raw: map[string]any{
				"openapi": version,
				"info":    map[string]any{"title": "Versioned", "version": "1"},
				"paths": map[string]any{
					"x-routing": "metadata only",
					"/widgets":  map[string]any{"query": operationFixture("queryWidgets")},
				},
			}}
			if _, err := Build(document); err == nil || !strings.Contains(err.Error(), "/paths/~1widgets/query") {
				t.Fatalf("Build error = %v", err)
			}
		})
	}
	document := &openapidoc.Document{Version: openapidoc.Version32, Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Extensions", "version": "1"},
		"paths": map[string]any{
			"x-routing": "metadata only",
			"/widgets":  map[string]any{"query": operationFixture("queryWidgets")},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 1 || model.Operations[0].OperationID != "queryWidgets" {
		t.Fatalf("operations = %#v", model.Operations)
	}
}

func TestBuildResolvesLocalPathItemReferences(t *testing.T) {
	operation := operationFixture("getSharedItem")
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Referenced", "version": "1.0.0"},
		"paths": map[string]any{
			"/items/{itemID}": map[string]any{"$ref": "#/components/pathItems/Item", "summary": "Override"},
		},
		"components": map[string]any{
			"pathItems": map[string]any{
				"Item": map[string]any{"get": operation, "summary": "Shared"},
			},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 1 || model.Operations[0].OperationID != "getSharedItem" {
		t.Fatalf("operations = %#v", model.Operations)
	}
	if model.Operations[0].PathItemRaw["summary"] != "Override" {
		t.Fatalf("path item siblings not applied: %#v", model.Operations[0].PathItemRaw)
	}
}

func TestBuildResolvesEscapedLocalPathItemReferences(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Referenced", "version": "1.0.0"},
		"paths": map[string]any{
			"/items": map[string]any{"$ref": "#/components/pathItems/Items~1Shared"},
		},
		"components": map[string]any{
			"pathItems": map[string]any{
				"Items/Shared": map[string]any{"get": operationFixture("getSharedItems")},
			},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 1 || model.Operations[0].OperationID != "getSharedItems" {
		t.Fatalf("operations = %#v", model.Operations)
	}
}

func TestBuildResolvesLocalPathsPathItemReferences(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Referenced", "version": "1.0.0"},
		"paths": map[string]any{
			"/common": map[string]any{"get": operationFixture("getCommon")},
			"/alias":  map[string]any{"$ref": "#/paths/~1common"},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Operations) != 2 || model.Operations[1].OperationID != "getCommon" {
		t.Fatalf("operations = %#v", model.Operations)
	}
}

func TestResolvePathItemDecodesURIFragmentBeforeJSONPointer(t *testing.T) {
	document := map[string]any{
		"paths": map[string]any{
			"/shared%20hook": map[string]any{"get": map[string]any{"operationId": "shared"}},
			"":               map[string]any{"post": map[string]any{"operationId": "empty"}},
		},
	}
	for reference, method := range map[string]string{
		"#%2Fpaths%2F~1shared%2520hook": "get",
		"#/paths/":                      "post",
	} {
		resolved, err := ResolvePathItem(document, map[string]any{"$ref": reference})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := resolved[method].(map[string]any); !ok {
			t.Fatalf("resolved %s path item = %#v", reference, resolved)
		}
	}
}

func TestResolvePathItemMarksCyclicReferenceErrors(t *testing.T) {
	loop := map[string]any{"$ref": "#/components/pathItems/Loop"}
	document := map[string]any{
		"components": map[string]any{
			"pathItems": map[string]any{"Loop": loop},
		},
	}
	_, err := ResolvePathItem(document, loop)
	if err == nil || !IsReferenceError(err) || !strings.Contains(err.Error(), "cyclic path item reference") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildRegistersLosslessSchemaResources(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi":           "3.2.0",
		"$self":             "https://api.example.test/openapi.json",
		"jsonSchemaDialect": "https://example.test/dialect/root",
		"info":              map[string]any{"title": "Schemas", "version": "1"},
		"paths":             map[string]any{},
		"components": map[string]any{"schemas": map[string]any{
			"Never": false,
			"Thing": map[string]any{"$id": "schemas/thing", "$schema": "https://example.test/dialect/thing", "type": "object"},
		}},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	never, ok := model.Schemas["Never"]
	if !ok || never.Value != false || never.Pointer != "/components/schemas/Never" || never.Dialect != "https://example.test/dialect/root" {
		t.Fatalf("Never schema = %#v", never)
	}
	thing := model.Schemas["Thing"]
	if thing.ResourceURI != "https://api.example.test/schemas/thing" || thing.Dialect != "https://example.test/dialect/thing" {
		t.Fatalf("Thing schema = %#v", thing)
	}
}

func operationFixture(operationID string) map[string]any {
	return map[string]any{
		"operationId":      operationID,
		"x-envelope":       "data",
		"x-concurrency":    "none",
		"x-idempotency":    "unsupported",
		"x-pagination":     map[string]any{"mode": "cursor"},
		"x-sdk-visibility": "public",
	}
}
