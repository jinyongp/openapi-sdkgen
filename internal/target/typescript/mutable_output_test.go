package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestMutableOutputProjectionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
"openapi":"3.2.0","info":{"title":"Mutable output","version":"1"},
"components":{"schemas":{
"Model":{"type":"object","required":["name","profile","tags","tuple","map","patterns","status","id","secret"],"properties":{"name":{"type":"string"},"profile":{"type":"object","required":["city"],"properties":{"city":{"type":"string"}},"additionalProperties":false},"tags":{"type":"array","items":{"type":"string"}},"tuple":{"type":"array","prefixItems":[{"type":"string"},{"type":"integer"}],"items":false},"map":{"type":"object","additionalProperties":{"$ref":"#/components/schemas/Entry"}},"patterns":{"type":"object","patternProperties":{"^x":{"$ref":"#/components/schemas/Entry"}},"additionalProperties":false},"status":{"type":"string","enum":["draft","done"]},"id":{"type":"string","readOnly":true},"secret":{"type":"string","writeOnly":true},"next":{"$ref":"#/components/schemas/Model"},"nullable":{"type":["string","null"]}},"additionalProperties":false},
"Entry":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}},
"Composite":{"allOf":[{"$ref":"#/components/schemas/Model"},{"type":"object","required":["extra"],"properties":{"extra":{"type":"string"}}}]},
"Failure":{"type":"object","required":["error"],"properties":{"error":{"type":"object","required":["code","details"],"properties":{"code":{"const":"BAD"},"details":{"$ref":"#/components/schemas/Model"}}}}}
}},"paths":{
"/model":{"post":{"operationId":"model","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Model"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Model"}}}}}}},
"/stream":{"get":{"operationId":"stream","responses":{"200":{"description":"ok","content":{"application/x-ndjson":{"itemSchema":{"$ref":"#/components/schemas/Model"}}}}}}},
"/composite":{"get":{"operationId":"composite","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Composite"}}}}}}},
"/error":{"get":{"operationId":"error","responses":{"400":{"description":"error","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Failure"}}}}}}},
"/pages":{"get":{"operationId":"pages","parameters":[{"name":"cursor","in":"query","schema":{"type":"string"}},{"name":"limit","in":"query","schema":{"type":"integer","minimum":1}}],"x-pagination":"cursor","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/Model"}},"pagination":{"type":"object","properties":{"nextCursor":{"type":["string","null"]}}}}}}}}}}}
},"webhooks":{"model":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Model"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Model"}}}}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	options, _ := registry.Resolve([]string{"server", "metadata"})
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	probe := `import {createClient,type ComponentInput,type ComponentOutput,type RouteContract,type ServerErrorDetailsByCode} from './index.js';
import {createWebhookRouter} from './server/webhooks.js';
import {openapi} from './metadata.js';
type Model = ComponentOutput<'Model'>;
type Input = ComponentInput<'Model'>;
interface Form { name:string;profile:{city:string};tags:string[];tuple:[string,number,...never[]];map:Record<string,{id:string}>; }
declare const model: Model;
model.name='edited'; model.id='server-readOnly-is-editable-in-output'; model.profile.city='Busan';
model.tags.push('new'); model.tuple[1]=4; model.map['entry']!.id='changed';
model.patterns['x-entry']={id:'new'}; model.patterns['x-entry']!.id='changed';
model.next=model; model.next.name='recursive'; model.nullable=null; model.status='done';
const form: Form = model; void form;
// @ts-expect-error literal enum remains constrained
model.status='unknown';
// @ts-expect-error nullable field remains string or null
model.nullable=3;
// @ts-expect-error writeOnly property stays absent from output
model.secret;
declare const input: Input;
// @ts-expect-error input properties keep their readonly contract
input.name='changed';
// @ts-expect-error readonly input arrays still accept immutable values
input.tags.push('changed');
// @ts-expect-error readOnly property stays absent from input
input.id;
const readonlyTags: readonly string[] = ['one'];
const compatibleInput: Input = {...input,tags:readonlyTags}; void compatibleInput;
declare const readonlyMock: Omit<Model,'tags'> & {readonly tags:readonly string[]};
// @ts-expect-error immutable response mocks need mutable arrays
const incompatibleOutput: Model = readonlyMock; void incompatibleOutput;
declare const failure: Extract<RouteContract<'GET /error'>['error'],{code:'BAD'}>;
if (failure.details !== undefined) { failure.details.tags.push('error'); failure.details.profile.city='new'; }
declare const globalDetails: ServerErrorDetailsByCode['BAD']; globalDetails.tags.push('global');
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
async function calls(): Promise<void> {
  const normal: Model = await api.$operations.model({body:input}); normal.tags.push('new');
  const raw: Awaited<ReturnType<typeof api.$operations.model.raw>> = await api.$operations.model.raw({body:input});
  raw.data.tags.push('raw'); raw.data.profile.city='new';
  for await(const item of api.$operations.stream.stream()) { item.tags.push('stream'); item.profile.city='new'; }
  for await(const item of api.$operations.pages.paginate({})) { item.tags.push('page'); item.profile.city='new'; }
  const composite: ComponentOutput<'Composite'> = await api.$operations.composite(); composite.extra='new'; composite.tags.push('new');
}
void calls;
createWebhookRouter({model:{POST:async()=>({status:200,body:model})}},{routes:{model:'/model'}});
// @ts-expect-error server output requires mutable body arrays
createWebhookRouter({model:{POST:async()=>({status:200,body:readonlyMock})}},{routes:{model:'/model'}});
// @ts-expect-error client operation catalog stays readonly
api.$operations.model=api.$operations.model;
// @ts-expect-error metadata stays readonly
openapi.version='3.2.0';
`
	compileDeclarationConsumers(t, artifacts, probe)
}
