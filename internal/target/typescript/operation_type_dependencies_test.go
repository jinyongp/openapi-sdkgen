package typescript

import (
	"testing"
)

func TestOperationTypeHelpersUseLocalContracts(t *testing.T) {
	module := operationModulePlan{routeKey: "GET /one", path: "internal/operations/one/get.ts"}
	plan := &semanticModulePlan{
		operationByRoute:       map[string]string{"GET /one": module.path, "GET /two": "internal/operations/two/get.ts"},
		operationByQuotedRoute: map[string]string{quoteTS("GET /one"): "GET /one", quoteTS("GET /two"): "GET /two"},
	}
	for _, test := range []struct{ source, want string }{
		{`RouteInput<"GET /one">`, `OperationPublicType<Input>`},
		{`RouteOptions<RouteKey>`, `OperationPublicType<Options>`},
		{`RouteOutput<"GET /two">`, `OperationPublicType<import("../two/get.js").Output>`},
		{`RouteRawResponse<RouteKey>`, `OperationPublicType<RawResponse>`},
		{`RouteResourceInput<RouteKey>`, `OperationPublicType<ResourceInput>`},
		{`OperationRawCall<RouteKey>`, `(RawCall & RouteTypeIdentity<RouteKey>)`},
		{`ResourceRawCapability<RouteKey>`, `ResourceRawCapability<RouteKey>`},
		{`StreamCall<RouteKey>`, `(Stream & RouteTypeIdentity<RouteKey>)`},
		{`PaginateCall<RouteKey>`, `(Pagination & RouteTypeIdentity<RouteKey>)`},
		{`LinkCalls<RouteKey>`, `(Links & RouteTypeIdentity<RouteKey>)`},
		{`type T = RouteInput < "GET /one" >`, `type T = OperationPublicType<Input>`},
	} {
		actual, err := localizeOperationHelperTypes(test.source, module, plan)
		if err != nil || actual != test.want {
			t.Fatalf("%s: got %q / %v; want %q", test.source, actual, err, test.want)
		}
	}
	for _, source := range []string{
		`type T = OtherRouteInput<"GET /one">`,
		`type T = δRouteInput<"GET /one">`,
		`type T = NS.RouteInput<"GET /one">`,
		`type T = NS. RouteInput<"GET /one">`,
		`type T = RouteInput<"GET /unknown">`,
		`type T = RouteInput<RouteKeyOther>`,
		`const text = "RouteInput<\"GET /one\">"`,
		"const text = `RouteInput<\"GET /one\">`",
		`// RouteInput<"GET /one">`,
		`/* RouteInput<"GET /one"> */`,
	} {
		actual, err := localizeOperationHelperTypes(source, module, plan)
		if err != nil || actual != source {
			t.Fatalf("non-helper source changed: %q -> %q (%v)", source, actual, err)
		}
	}
}
