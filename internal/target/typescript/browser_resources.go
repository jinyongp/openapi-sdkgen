package typescript

import (
	"bytes"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// Browser resource placement uses the already reconciled full-document tree.
// Removing an operation must never rename or resurrect a conflicting resource.
type browserResourcePlacement struct {
	Path           []any    `json:"path"`
	Member         string   `json:"member"`
	PathParameters []string `json:"pathParameters,omitempty"`
	HasInput       bool     `json:"hasInput,omitempty"`
	InputOptional  bool     `json:"inputOptional,omitempty"`
	Pagination     bool     `json:"pagination,omitempty"`
}

func browserResourcePlacements(root *resourceNode) map[string][]browserResourcePlacement {
	result := make(map[string][]browserResourcePlacement)
	var visit func(*resourceNode, []any)
	visit = func(node *resourceNode, path []any) {
		for _, name := range sortedResourceMemberNames(node) {
			operation, exists := node.operations[name]
			if !exists {
				continue
			}
			hasInput := operation.InputSections.hasInput(true)
			placement := browserResourcePlacement{
				Path: append([]any{}, path...), Member: name,
				PathParameters: append([]string(nil), operation.PathParameterOrder...),
				HasInput:       hasInput, InputOptional: hasInput && !operation.prepared.resourceInputRequired,
			}
			route := manifestRouteKey(operation)
			result[route] = append(result[route], placement)
		}
		if operation, exists := paginatedResourceNodeOperation(node); exists {
			route := manifestRouteKey(operation)
			result[route] = append(result[route], browserResourcePlacement{
				Path: append([]any{}, path...), Member: "paginate", Pagination: true,
			})
		}
		for _, name := range sortedResourceChildNames(node) {
			visit(node.children[name], append(append([]any{}, path...), name))
		}
		if node.parameterChild != nil {
			visit(node.parameterChild, append(append([]any{}, path...), nil))
		}
	}
	visit(root, nil)
	return result
}

// Each node's type is parameterized by guaranteed and possible routes. Sharing
// child aliases avoids expanding every ancestor's complete descendant union.
func emitSelectedResourceTypes(document *ir.Document, plan *semanticModulePlan, root *resourceNode) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString(`type Member<Key extends string, Routes extends RouteKey, Guaranteed extends RouteKey, Possible extends RouteKey, Value> =
  [Extract<Routes, Possible>] extends [never] ? {} :
  [Extract<Routes, Guaranteed>] extends [never] ? { readonly [K in Key]?: Value } : { readonly [K in Key]: Value }
// A parameter builder or operation may share a namespace. When only its
// descendants are guaranteed, the callable itself must still be narrowed.
type WithCall<Call, CallRoutes extends RouteKey, Members, MemberRoutes extends RouteKey, Guaranteed extends RouteKey, Possible extends RouteKey> =
  [Extract<CallRoutes, Possible>] extends [never] ? Members :
  [Extract<MemberRoutes, Possible>] extends [never] ? Call :
  [Extract<CallRoutes, Guaranteed>] extends [never] ? Members | (Call & Members) : Call & Members
// Distribute over each member's finite route set, not the whole document for
// every property. Required/possible membership is unchanged for generated keys.
type SelectedMembers<Values, Routes extends { readonly [Key in keyof Values]: RouteKey }, G extends RouteKey, P extends RouteKey> = {
  readonly [Key in keyof Values as [Extract<Routes[Key], G>] extends [never] ? never : Key]: Values[Key]
} & {
  readonly [Key in keyof Values as [Extract<Routes[Key], P>] extends [never] ? never : [Extract<Routes[Key], G>] extends [never] ? Key : never]?: Values[Key]
}

`)
	ids := make(map[*resourceNode]int)
	var nodes []*resourceNode
	var collect func(*resourceNode)
	collect = func(node *resourceNode) {
		ids[node] = len(nodes)
		nodes = append(nodes, node)
		for _, name := range sortedResourceChildNames(node) {
			collect(node.children[name])
		}
		if node.parameterChild != nil {
			collect(node.parameterChild)
		}
	}
	collect(root)
	nodeRoutes := func(node *resourceNode) string { return fmt.Sprintf("NodeRoutes%d", ids[node]) }
	nodeType := func(node *resourceNode) string { return fmt.Sprintf("Node%d<G, P>", ids[node]) }
	operationType := func(route, slot string) (string, error) {
		module, exists := plan.operationByRoute[route]
		if !exists {
			return "", fmt.Errorf("selected resource route %q has no operation module", route)
		}
		specifier, err := plan.relativeModuleSpecifier("browser/types.ts", module)
		if err != nil {
			return "", err
		}
		if slot == "ResourceCall" {
			return "import(" + quoteTS(specifier) + ").ResourceMethod<" + quoteTS(route) + ">", nil
		}
		return "import(" + quoteTS(specifier) + ")." + slot, nil
	}
	for _, node := range nodes {
		members := make([]string, 0)
		memberRouteFields := make([]string, 0)
		routes := make([]string, 0)
		for _, name := range sortedResourceMemberNames(node) {
			operation, hasOperation := node.operations[name]
			child := node.children[name]
			memberRoutes, memberValue := "never", "{}"
			if child != nil {
				memberRoutes, memberValue = nodeRoutes(child), nodeType(child)
			}
			if hasOperation {
				route := manifestRouteKey(operation)
				call, err := operationType(route, "ResourceCall")
				if err != nil {
					return nil, err
				}
				if child == nil {
					memberValue = call
				} else {
					memberValue = "WithCall<" + call + ", " + quoteTS(route) + ", " + memberValue + ", " + memberRoutes + ", G, P>"
				}
				memberRoutes += " | " + quoteTS(route)
			}
			routes = append(routes, memberRoutes)
			members = append(members, "  readonly "+quoteTS(name)+": "+memberValue)
			memberRouteFields = append(memberRouteFields, "  readonly "+quoteTS(name)+": "+memberRoutes)
		}
		if operation, exists := paginatedResourceNodeOperation(node); exists {
			route := manifestRouteKey(operation)
			call, err := operationType(route, "Pagination")
			if err != nil {
				return nil, err
			}
			routes = append(routes, quoteTS(route))
			members = append(members, "  readonly paginate: "+call)
			memberRouteFields = append(memberRouteFields, "  readonly paginate: "+quoteTS(route))
		}
		memberRoutes := "never"
		if len(routes) > 0 {
			memberRoutes = strings.Join(routes, " | ")
		}
		memberValue := "{}"
		if len(members) > 0 {
			fmt.Fprintf(&output, "interface NodeMembers%d<G extends RouteKey, P extends RouteKey> {\n%s\n}\n", ids[node], strings.Join(members, "\n"))
			fmt.Fprintf(&output, "interface NodeMemberRoutes%d {\n%s\n}\n", ids[node], strings.Join(memberRouteFields, "\n"))
			memberValue = fmt.Sprintf("SelectedMembers<NodeMembers%d<G, P>, NodeMemberRoutes%d, G, P>", ids[node], ids[node])
		}
		nodeValue := memberValue
		allRoutes := memberRoutes
		if child := node.parameterChild; child != nil {
			parameterType, err := resourceParameterType(document, plan, "browser/types.ts", child.parameter)
			if err != nil {
				return nil, err
			}
			call := "((value: " + parameterType + ") => " + nodeType(child) + ")"
			nodeValue = "WithCall<" + call + ", " + nodeRoutes(child) + ", " + memberValue + ", " + memberRoutes + ", G, P>"
			allRoutes += " | " + nodeRoutes(child)
		}
		fmt.Fprintf(&output, "type NodeRoutes%d = %s\n", ids[node], allRoutes)
		fmt.Fprintf(&output, "type Node%d<G extends RouteKey, P extends RouteKey> = %s\n\n", ids[node], nodeValue)
	}
	output.WriteString("export type SelectedResources<G extends RouteKey, P extends RouteKey> = Node0<G, P>\n")
	return output.Bytes(), nil
}
