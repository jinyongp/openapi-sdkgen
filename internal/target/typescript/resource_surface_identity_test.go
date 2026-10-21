package typescript

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestResourceSurfaceReusesCanonicalParameterCall(t *testing.T) {
	path := "/roots/{id0}"
	parameters := []any{map[string]any{"name": "id0", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}}
	call := `api.roots("zero")`
	for depth := 1; depth <= 22; depth++ {
		name := fmt.Sprintf("id%d", depth)
		path += "/next/{" + name + "}"
		parameters = append(parameters, map[string]any{"name": name, "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		call += fmt.Sprintf(".next(%q)", name)
	}
	input := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Canonical resource surface", "version": "1"}, "paths": map[string]any{path: map[string]any{"get": map[string]any{"operationId": "read", "parameters": parameters, "responses": map[string]any{"200": map[string]any{"description": "Done", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "string"}}}}}}}}}
	paths := input["paths"].(map[string]any)
	for index := range 80 {
		paths[fmt.Sprintf("/branch%d/{key}", index)] = map[string]any{"get": map[string]any{"operationId": fmt.Sprintf("branch%d", index), "parameters": []any{map[string]any{"name": "key", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}}}, "responses": map[string]any{"200": map[string]any{"description": "Done", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "integer"}}}}}}}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil || diagnostic.HasErrors(values) {
		t.Fatalf("generate: %v %v", err, values)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	probe := strings.Replace(resourceSurfaceIdentityConsumer, "CHAIN", call, 1)
	compileDeclarationConsumers(t, artifacts, probe)
}

const resourceSurfaceIdentityConsumer = `import type {Client} from './index.js';
import type {Surface as Collection} from './internal/resources/roots/index.js';
import type {Surface as Item} from './internal/resources/roots/by-id0.js';
import {build} from './internal/resources/root.js';
import type {RegistryCalls} from './internal/resources/root.js';
import {build as buildBranch} from './internal/resources/branch0/index.js';
type LegacyRoot = ((id0:string)=>Item)&Collection;
type Expect<Value extends true> = Value;
type Equal<Left,Right> = (<Value>()=>Value extends Left?1:2) extends (<Value>()=>Value extends Right?1:2)?true:false;
export type Signature = [Expect<Equal<Parameters<Client['roots']>,Parameters<LegacyRoot>>>,Expect<Equal<ReturnType<Client['roots']>,ReturnType<LegacyRoot>>>,Expect<Equal<keyof Client['roots'],keyof LegacyRoot>>];
export function toLegacy<Value extends Client['roots']>(value:Value):LegacyRoot{return value}
export function fromLegacy<Value extends LegacyRoot>(value:Value):Client['roots']{return value}
export function parentRegistry<Value extends Parameters<typeof build>[0]>(value:Value):Parameters<typeof buildBranch>[0]{return value}
declare const missing:Omit<RegistryCalls,'GET /branch0/{key}'>;
// @ts-expect-error the inherited registry still requires every exact operation
build({routes:missing});
declare const api:Client;
const branchResponse=await api.branch0(123).get.raw();
const branchValue:number=branchResponse.data;
void branchValue;
// @ts-expect-error sibling path scalar remains an integer
api.branch0('wrong');
const bound:()=>Item=api.roots.bind(undefined,"zero");
const called:Item=api.roots.call(undefined,"zero");
void bound;void called;
const response=await CHAIN.get.raw();
const value:string=response.data;
void value;
// @ts-expect-error parameter scalar remains a string
api.roots(123);
// @ts-expect-error Function.bind preserves the path parameter scalar
api.roots.bind(undefined,123);
// @ts-expect-error Function.call preserves the resource return type
const wrong:number=api.roots.call(undefined,"zero");
void wrong;
// @ts-expect-error collection has no operation before all path parameters bind
api.roots.get;
`
