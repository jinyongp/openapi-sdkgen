package typescript

import (
	"fmt"
	"os"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestWildcardRequestBodyCommentsCompile(t *testing.T) {
	input, err := os.ReadFile("../../../test/fixtures/wildcard-request-body.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, typecheck := range []bool{false, true} {
		t.Run(fmt.Sprintf("typecheck=%t", typecheck), func(t *testing.T) {
			artifacts, err := (Generator{}).Generate(document, generator.Options{TypeScriptTypeCheck: &typecheck})
			if err != nil {
				t.Fatal(err)
			}
			source := operationArtifactSource(t, artifacts, "POST /proxy")
			if strings.Contains(source, "// @ts-nocheck\n") == typecheck {
				t.Fatal("generated source does not follow the typecheck policy")
			}
			compileTypeScriptArtifactSet(t, artifacts, "wildcard.probe.ts", `import type { OperationInput } from './index.js';
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
interface ExpectedBody { readonly contentType: "*/*"; readonly value: unknown }
type Body = NonNullable<OperationInput<"postProxy">["body"]>;
export const exactBody: Equal<Body, ExpectedBody> = true;
export const optionalBody: OperationInput<"postProxy"> = {};
export const opaqueBody: OperationInput<"postProxy"> = { body: { contentType: "*/*", value: new Uint8Array([0, 127, 255]) } };
`)
		})
	}
}

func TestGeneratedCommentsEscapeSourceText(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Comment safety", "version": "1"},
  "paths": {
    "/comments*/{id}": {"post": {
      "operationId": "comment*/operation",
      "summary": "Summary */ text",
      "description": "Description */ text",
      "parameters": [
        {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
        {"name": "query*/name", "in": "query", "description": "Query */ text", "schema": {"type": "string", "default": "*/"}}
      ],
      "requestBody": {"content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "property*/name": {"type": "string", "default": "*/", "enum": ["*/"], "description": "Property */ text"},
          "nested": {"type": "object", "default": {"marker": "*/"}}
        }
      }}}},
      "responses": {"200": {"description": "Response */ text", "content": {"application/json": {"schema": {"type": "string", "enum": ["*/"]}}}}},
      "callbacks": {"notify": {"https://example.test/hooks*/done": {"post": {"responses": {"204": {"description": "OK"}}}}}}
    }},
    "/no-id*/route": {"get": {"responses": {"204": {"description": "OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := sourceArtifacts(document, true)
	if err != nil {
		t.Fatal(err)
	}
	compileTypeScriptArtifactSet(t, artifacts, "comments.probe.ts", `import type { OperationInput, OperationOutput } from './index.js';
export const input: OperationInput<"comment*/operation"> = {
  path: { id: "item" }, query: { "query*/name": "*/" }, body: { "property*/name": "*/" }
};
export const output: OperationOutput<"comment*/operation"> = "*/";
`)
}
