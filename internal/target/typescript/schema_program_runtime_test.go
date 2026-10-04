package typescript

import (
	"encoding/json"
	"os/exec"
	"sort"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

// The compatibility oracle and generated programs run from the same source basis.
func TestGeneratedSchemaProgramsMatchCompatibilitySemantics(t *testing.T) {
	schemas := map[string]any{
		"BooleanFalse":          false,
		"BooleanTrue":           true,
		"Number":                map[string]any{"type": "number", "minimum": -1, "exclusiveMaximum": 3, "multipleOf": 0.1},
		"Integer":               map[string]any{"type": "integer"},
		"ReferenceObjectNumber": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/Number"}}},
		"ReferenceObjectString": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/String"}}},
		"AnchorOnly":            map[string]any{"type": "integer", "$dynamicAnchor": "number"},
		"String":                map[string]any{"type": "string", "minLength": 1, "maxLength": 3, "pattern": "^[a-z😀]+$"},
		"Literal":               map[string]any{"enum": []any{nil, "a", 1, map[string]any{"x": 1}}},
		"Object":                map[string]any{"type": "object", "required": []any{"x"}, "properties": map[string]any{"x": map[string]any{"type": "integer"}, "read": map[string]any{"type": "string", "readOnly": true}, "write": map[string]any{"type": "string", "writeOnly": true}}, "additionalProperties": false},
		"Additional":            map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "number"}}, "additionalProperties": map[string]any{"type": "string"}},
		"Pattern":               map[string]any{"type": "object", "patternProperties": map[string]any{"^x": map[string]any{"type": "integer"}, "x$": map[string]any{"minimum": 1}}, "additionalProperties": false},
		"Dependencies":          map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{}, "y": map[string]any{}}, "dependentRequired": map[string]any{"x": []any{"y"}}, "dependentSchemas": map[string]any{"y": map[string]any{"properties": map[string]any{"x": map[string]any{"type": "integer"}}}}, "propertyNames": map[string]any{"maxLength": 1}},
		"Array":                 map[string]any{"type": "array", "prefixItems": []any{map[string]any{"type": "integer"}}, "items": map[string]any{"type": "string"}, "minItems": 1, "maxItems": 3, "uniqueItems": true},
		"Contains":              map[string]any{"type": "array", "contains": map[string]any{"type": "integer"}, "minContains": 1, "maxContains": 2, "unevaluatedItems": false},
		"OneOf":                 map[string]any{"oneOf": []any{map[string]any{"type": "object", "required": []any{"x"}, "properties": map[string]any{"x": map[string]any{"type": "integer"}}, "additionalProperties": false}, map[string]any{"type": "object", "required": []any{"y"}, "properties": map[string]any{"y": map[string]any{"type": "string"}}, "additionalProperties": false}}},
		"AnyOf":                 map[string]any{"anyOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "number", "minimum": 1}}},
		"AllOf":                 map[string]any{"type": "object", "allOf": []any{map[string]any{"properties": map[string]any{"x": map[string]any{"type": "integer"}}}, map[string]any{"properties": map[string]any{"y": map[string]any{"type": "string"}}}}, "unevaluatedProperties": false},
		"Conditional":           map[string]any{"if": map[string]any{"type": "integer"}, "then": map[string]any{"minimum": 1}, "else": map[string]any{"type": "string"}, "not": map[string]any{"const": "bad"}},
		"Content":               map[string]any{"type": "string", "contentMediaType": "application/json", "contentSchema": map[string]any{"type": "object", "required": []any{"x"}, "properties": map[string]any{"x": map[string]any{"type": "integer"}}}},
		"ContentBase64":         map[string]any{"type": "string", "contentEncoding": "base64", "contentMediaType": "application/json", "contentSchema": map[string]any{"type": "integer"}},
		"Format":                map[string]any{"type": "string", "format": "ipv4", "$vocabulary": map[string]any{formatAssertionVocabulary: true}},
		"Inert":                 map[string]any{"type": "string", "properties": map[string]any{"x": map[string]any{"type": "integer"}}, "items": map[string]any{"type": "number"}, "additionalProperties": false},
		"Prototype":             map[string]any{"type": "object", "required": []any{"__proto__"}, "properties": map[string]any{"__proto__": map[string]any{"type": "integer"}, "constructor": map[string]any{"type": "string"}}, "additionalProperties": false},
		"Node":                  map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "integer"}, "children": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Node"}}}},
	}
	var variants []any
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		variants = append(variants, map[string]any{"$ref": "#/components/schemas/" + name})
	}
	schemas["Probe"] = map[string]any{"anyOf": variants}
	content := map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Probe"}}}
	operation := map[string]any{"operationId": "probe", "requestBody": map[string]any{"content": content}, "responses": map[string]any{"200": map[string]any{"description": "ok", "content": content}}}
	data, err := json.Marshal(map[string]any{"openapi": "3.1.1", "info": map[string]any{"title": "Programs", "version": "1"}, "paths": map[string]any{"/probe": map[string]any{"post": operation}}, "components": map[string]any{"schemas": schemas}})
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
	seen := make(map[string]bool)
	for _, artifact := range artifacts {
		seen[artifact.Path] = true
	}
	compatibility, err := emitPreparedRuntimeTemplateArtifacts(runtimeTemplateArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range compatibility {
		if !seen[artifact.Path] {
			artifacts = append(artifacts, artifact)
		}
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	script := `
import assert from 'node:assert/strict'; import {pathToFileURL} from 'node:url';
const root=process.argv[1]; const load=file=>import(pathToFileURL(root+'/'+file));
const {inputSchemas,outputSchemas}=await load('internal/schemas/wire.js');
const {createProgramCodec}=await load('internal/runtime/schema/program-codec.js');
const {fullWireHandlers}=await load('internal/runtime/compatibility/wire-handlers.js');
const {jsonWireCodec}=await load('internal/runtime/compatibility/wire-engine.js');
const generated=createProgramCodec(fullWireHandlers);
const values=[undefined,null,true,false,-2,-1,0,0.1,1,1.5,2,3,NaN,Infinity,'','a','bad','😀','abcd','127.0.0.1','{"x":1}','{"x":"a"}','MQ==',[],[1],[1,'a'],[1,'a','a'],[1,2,3],[1,undefined],new Array(1),{}, {value:1},{value:'a'},{x:1},{x:0},{x:'a'},{y:'a'},{x:1,y:'a'},{x:1,extra:1},{x:1,extra:Infinity},{x:1,write:'a',read:'a'},JSON.parse('{"__proto__":1,"constructor":"a"}'),Object.create({x:1}),{x:1,children:[{x:2}]}];
function outcome(codec,value,schema,schemas,direction,options) {
 try { return {accepted:true,value:codec.transformWireValue(value,schema,schemas,direction,options)} }
 catch(error) { return {accepted:false,type:error.constructor.name} }
}
let comparisons=0;
for(const [direction,schemas] of [['encode',inputSchemas],['decode',outputSchemas]]){
 for(const [name,schema] of Object.entries(schemas)) {
  for(const value of values) for(const unknownProperties of ['reject','preserve']) {
   const before=structuredClone(value);
   const expected=outcome(jsonWireCodec,value,schema,schemas,direction,{unknownProperties});
   const actual=outcome(generated,value,schema,schemas,direction,{unknownProperties});
   assert.deepEqual(actual,expected,JSON.stringify({name,direction,unknownProperties,value}));
   assert.deepEqual(structuredClone(value),before,'source mutated'); comparisons++;
  }
 }
}
assert.throws(()=>generated.validateWireValue(1,{types:['integer']},{},'encode'),/missing generated schema program/);
const reference=Object.values(inputSchemas).find(schema=>schema.properties?.children?.schema.items?.reference==='Node');
assert(reference);
assert.throws(()=>generated.transformWireValue({children:[{}]},reference,{},'encode'),/missing generated schema reference/);
console.log('schema differential comparisons',comparisons);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("schema program differential: %v\n%s", err, result)
	}
}
