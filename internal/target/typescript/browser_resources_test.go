package typescript

import (
	"fmt"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

// Wide resource namespaces used to build an intersection for every child. The
// deferred mapped-member representation must remain independent of that width.
func TestSelectedResourceTypesUseDeferredMemberMaps(t *testing.T) {
	for _, count := range []int{100, 1000, 10000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var input strings.Builder
			input.WriteString(`{"openapi":"3.1.1","info":{"title":"Wide resources","version":"1"},"paths":{`)
			for index := 0; index < count; index++ {
				if index != 0 {
					input.WriteByte(',')
				}
				fmt.Fprintf(&input, `"/items/item%d":{"get":{"operationId":"getItem%d","responses":{"204":{"description":"ok"}}}}`, index, index)
			}
			input.WriteString(`}}`)
			document, err := sdkgen.Compile([]byte(input.String()))
			if err != nil {
				t.Fatal(err)
			}
			plan, diagnostics, err := prepareSourcePlan(document, false)
			if err != nil || diagnostic.HasErrors(diagnostics) {
				t.Fatalf("prepare: %v %v", err, diagnostics)
			}
			source, err := emitSelectedResourceTypes(plan.document, plan.modules, plan.resourceTree)
			if err != nil {
				t.Fatal(err)
			}
			text := string(source)
			if !strings.Contains(text, "SelectedMembers<NodeMembers1<G, P>, NodeMemberRoutes1, G, P>") {
				t.Fatal("wide namespace no longer uses deferred member lookup")
			}
			if strings.Contains(text, `Member<"item`) {
				t.Fatal("per-member conditional intersections returned")
			}
			browserTypes, err := emitBrowserTypes(plan)
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{
				"type SelectedIDs<Guaranteed extends RouteKey, Possible extends RouteKey>",
				"type SelectedLinkIDs<Guaranteed extends RouteKey, Possible extends RouteKey>",
				"SelectedIDs<G<Selection>, P<Selection>>",
				"SelectedLinkIDs<G<Selection>, P<Selection>>",
			} {
				if !strings.Contains(string(browserTypes), expected) {
					t.Fatalf("membership is no longer computed outside the mapped ID table: %s", expected)
				}
			}
			for _, index := range []int{0, count - 1} {
				if !strings.Contains(text, fmt.Sprintf(`readonly "item%d": Node`, index)) ||
					!strings.Contains(text, fmt.Sprintf(`"GET /items/item%d"`, index)) {
					t.Fatalf("resource or route disappeared: item%d", index)
				}
			}
		})
	}
}
