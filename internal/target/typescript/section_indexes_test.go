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

func TestSectionIndexesPreserveCanonicalGenericContracts(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Sections","version":"1"},"paths":{
"/things/{id}":{"get":{"operationId":"read","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"q","in":"query","schema":{"type":"integer"}},{"name":"X-Flag","in":"header","schema":{"type":"boolean"}},{"name":"session","in":"cookie","schema":{"type":"string"}}],"responses":{"204":{"description":"Done"}}}},
"/post":{"post":{"operationId":"post","requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"value":{"type":"string"}}}}}},"responses":{"204":{"description":"Done"}}}},
"/whole":{"get":{"operationId":"whole","parameters":[{"name":"whole","in":"querystring","content":{"application/json":{"schema":{"type":"object","additionalProperties":{"type":"string"}}}}}],"responses":{"204":{"description":"Done"}}}},
"/idless/{id}":{"get":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"other","in":"query","schema":{"type":"string"}}],"responses":{"204":{"description":"Done"}}}},
"/hidden":{"get":{"operationId":"hidden","x-sdk-visibility":"hidden","parameters":[{"name":"secret","in":"query","schema":{"type":"string"}}],"responses":{"204":{"description":"Done"}}}}
}}`
	var raw map[string]any
	if err := json.Unmarshal([]byte(source), &raw); err != nil {
		t.Fatal(err)
	}
	paths := raw["paths"].(map[string]any)
	for index := range 520 {
		paths[fmt.Sprintf("/padding%03d", index)] = map[string]any{"get": map[string]any{"operationId": fmt.Sprintf("padding%03d", index), "responses": map[string]any{"204": map[string]any{"description": "Done"}}}}
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil || diagnostic.HasErrors(values) {
		t.Fatalf("prepare: %v %v", err, values)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for index := range artifacts {
		if artifacts[index].Path != "internal/routes/helpers.ts" {
			continue
		}
		found = true
		text := string(artifacts[index].Data)
		if !strings.Contains(text, "RouteSourcesBySectionPart") {
			t.Fatal("fixture did not use section indexes")
		}
		// Keep the unchanged recursive value aliases canonical. Copying them to
		// another module makes generic comparisons fail even for identical filters.
		text += sectionIndexTestExports + strings.NewReplacer(
			"RouteSourceForSection", "LegacyRouteSourceForSection",
			"OperationIDSourceForSection", "LegacyOperationIDSourceForSection",
			"ResourceOperationRouteForSection", "LegacyResourceOperationRouteForSection",
		).Replace(legacySectionFilters) + legacySectionValueExports
		artifacts[index].Data = []byte(text)
		artifacts = append(artifacts, generator.Artifact{Path: "internal/routes/legacy-helpers.ts", Data: []byte(legacySectionFacade)})
		break
	}
	if !found {
		t.Fatal("route helpers absent")
	}
	var assertions strings.Builder
	for _, section := range []string{`"path"`, `"query"`, `"querystring"`, `"header"`, `"cookie"`, `"body"`, `"query" | "path"`, `"body" | "missing"`, `never`, `PropertyKey`, `unique symbol`} {
		if section == "unique symbol" {
			section = "typeof unknownSection"
		}
		for _, contract := range []string{"TestRouteSources", "TestOperationIDs", "TestResourceRoutes"} {
			fmt.Fprintf(&assertions, "Expect<Equal<Next.%s<%s>, Old.%s<%s>>>,\n", contract, section, contract, section)
		}
	}
	probe := strings.Replace(sectionIndexConsumer, "ASSERTIONS", assertions.String(), 1)
	compileDeclarationConsumerProfiles(t, artifacts, probe, []declarationConsumerProfile{{name: "nodenext", isolatedDeclarations: true}})
}

const sectionIndexTestExports = `
export type TestRouteSources<Section extends PropertyKey> = RouteSourceForSection<Section>
export type TestOperationIDs<Section extends PropertyKey> = OperationIDSourceForSection<Section>
export type TestResourceRoutes<Section extends PropertyKey> = ResourceOperationRouteForSection<Section>
`

const legacySectionFilters = `type RouteSourceForSection<Section extends PropertyKey> = {
  [Route in keyof RouteRequestSections]: Section extends keyof RouteRequestSections[Route] ? Route : never
}[keyof RouteRequestSections]
type OperationIDSourceForSection<Section extends PropertyKey> = {
  [ID in keyof OperationRoutes]: Section extends keyof RouteRequestSections[OperationRoutes[ID]] ? ID : never
}[keyof OperationRoutes]
type ResourceOperationRouteForSection<Section extends PropertyKey> = {
  [Route in keyof RouteRequestSections]: Routes[Route]["resourceCall"] extends never
    ? never : Section extends "path" ? never : Section extends keyof RouteRequestSections[Route] ? Route : never
}[keyof RouteRequestSections]

`

const legacySectionValueExports = `
type LegacyOperationSourceForSection<Section extends PropertyKey> =
  | LegacyOperationIDSourceForSection<Section>
  | OperationTypeIdentity<LegacyRouteSourceForSection<Section>, "exact">
  | OperationTypeIdentity<LegacyResourceOperationRouteForSection<Section>, "resource">
export type LegacyTestRouteSources<Section extends PropertyKey> = LegacyRouteSourceForSection<Section>
export type LegacyTestOperationIDs<Section extends PropertyKey> = LegacyOperationIDSourceForSection<Section>
export type LegacyTestResourceRoutes<Section extends PropertyKey> = LegacyResourceOperationRouteForSection<Section>
export type LegacyRouteQuery<Route extends LegacyRouteSourceForSection<"query">> = PublicType<SelectedRouteSection<Route,"query">>
export type LegacyOperationQuery<Source extends LegacyOperationSourceForSection<"query">> = PublicType<SelectedOperationSection<Source,"query">>
export type LegacyOperationBody<Source extends LegacyOperationSourceForSection<"body">> = PublicType<SelectedOperationSection<Source,"body">>
`

const legacySectionFacade = `export type {
  LegacyTestRouteSources as TestRouteSources,
  LegacyTestOperationIDs as TestOperationIDs,
  LegacyTestResourceRoutes as TestResourceRoutes,
  LegacyRouteQuery as RouteQuery,
  LegacyOperationQuery as OperationQuery,
  LegacyOperationBody as OperationBody,
} from './helpers.js';
`

const sectionIndexConsumer = `import type * as Next from './internal/routes/helpers.js';
import type * as Old from './internal/routes/legacy-helpers.js';
import type * as Public from './index.js';
type Expect<Value extends true> = Value;
type Equal<Left,Right> = (<Value>()=>Value extends Left?1:2) extends (<Value>()=>Value extends Right?1:2)?true:false;
declare const unknownSection: unique symbol;
export type Assertions = [
ASSERTIONS
Expect<Equal<Public.RouteQuery<'GET /things/{id}'>,Old.RouteQuery<'GET /things/{id}'>>>,
Expect<Equal<Public.RouteQuery<'GET /things/{id}'|'GET /idless/{id}'>,Old.RouteQuery<'GET /things/{id}'|'GET /idless/{id}'>>>,
Expect<Equal<Public.RouteQuery<never>,never>>,
Expect<Equal<Public.OperationQuery<never>,never>>,
Expect<Equal<Public.OperationBody<'post'>,Old.OperationBody<'post'>>>,
Expect<Equal<Public.OperationParameter<'read','query','q'>,number>>
];
export function routeGeneric<Route extends Old.TestRouteSources<'query'>>(old:Old.RouteQuery<Route>,next:Next.RouteQuery<Route>):[Old.RouteQuery<Route>,Next.RouteQuery<Route>]{return [next,old]}
export function operationGeneric<ID extends Old.TestOperationIDs<'query'>>(old:Old.OperationQuery<ID>,next:Next.OperationQuery<ID>):[Old.OperationQuery<ID>,Next.OperationQuery<ID>]{return [next,old]}
export function resourceGeneric<Route extends Old.TestResourceRoutes<'query'>>(old:Old.OperationQuery<Public.ResourceCall<Route>>,next:Next.OperationQuery<Public.ResourceCall<Route>>):[Old.OperationQuery<Public.ResourceCall<Route>>,Next.OperationQuery<Public.ResourceCall<Route>>]{return [next,old]}
const query:Public.RouteQuery<'GET /things/{id}'>={q:1};
// @ts-expect-error request sections remain readonly
query.q=2;
const body:Public.OperationBody<'post'>={value:'ok'};
const header:Public.OperationHeaders<'read'>={'X-Flag':true};
const cookie:Public.OperationCookies<'read'>={session:'ok'};
const whole:Public.RouteQuerystring<'GET /whole'>={whole:{search:'ok'}};
void query;void body;void header;void cookie;void whole;
// @ts-expect-error wrong query scalar
const wrong:Public.RouteQuery<'GET /things/{id}'>={q:'no'};
// @ts-expect-error missing section on padding operation
type MissingSection=Public.RouteQuery<'GET /padding000'>;
// @ts-expect-error hidden route
type HiddenRoute=Public.RouteQuery<'GET /hidden'>;
// @ts-expect-error no operation ID exists for an ID-less route
type IDLess=Public.OperationQuery<'GET /idless/{id}'>;
// @ts-expect-error resource path is already bound
type BoundPath=Public.OperationPath<Public.ResourceCall<'GET /things/{id}'>>;
// @ts-expect-error unknown parameter name
type UnknownParameter=Public.OperationParameter<'read','query','missing'>;
void (null as unknown as typeof wrong);
export type Negative=[MissingSection,HiddenRoute,IDLess,BoundPath,UnknownParameter];
`
