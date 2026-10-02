package typescript

import (
	"encoding/json"
	"fmt"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestPathResourceDoesNotReinstantiateDeepPublicResponse(t *testing.T) {
	schemas := make(map[string]any)
	for index := 0; index < 70; index++ {
		properties := map[string]any{"id": map[string]any{"type": "string"}}
		if index < 69 {
			properties["child"] = map[string]any{"$ref": fmt.Sprintf("#/components/schemas/Node%d", index+1)}
		}
		schemas[fmt.Sprintf("Node%d", index)] = map[string]any{
			"type": "object", "properties": properties, "required": []string{"id"}, "additionalProperties": false,
		}
	}
	input := map[string]any{
		"openapi": "3.1.1", "info": map[string]any{"title": "Deep resource types", "version": "1"},
		"components": map[string]any{"schemas": schemas},
		"paths": map[string]any{"/records/{id}": map[string]any{"get": map[string]any{
			"operationId": "getRecord",
			"parameters":  []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}},
			"responses": map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{
				"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Node0"}},
			}}},
		}}},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js";
const api = createClient({baseURL:"https://records.test"});
const response = await api.records("one").get.raw();
const id: string = response.data.id;
void id;
// @ts-expect-error The resource path parameter retains its public string type.
void api.records(123).get.raw();
// @ts-expect-error Deep public response fields retain their schema types.
const invalid: number = response.data.child?.id;
void invalid;
`
	compileTypeScriptArtifactSet(t, artifacts, "deep-resource.ts", probe)
}
