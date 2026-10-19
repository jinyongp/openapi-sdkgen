package golang

import (
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/http/extension"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type resourceBindingPlan struct{ Name, Key string }
type resourceMemberPlan struct{ Name, Target string }
type resourceCallPlan struct {
	Method    string
	Operation operationPlan
}
type resourceSelectorPlan struct {
	Name, Argument, Model, Validation, Key, Child string
	Parameter                                     ir.Parameter
}
type resourceNodePlan struct {
	Name, Identity string
	Bound          bool
	Children       []resourceMemberPlan
	Selector       *resourceSelectorPlan
	Calls          []resourceCallPlan
}
type resourceTree struct {
	identity           string
	depth              int
	children           map[string]*resourceTree
	parameter          *resourceTree
	selector           *resourceSelectorPlan
	parameterSignature string
	parameterBlocked   bool
	operations         []*operationPlan
}

func prepareResources(document *ir.Document, operations []operationPlan, names namePlan, schemas *schemaPlanner) ([]resourceNodePlan, error) {
	root := &resourceTree{identity: "/", children: make(map[string]*resourceTree)}
	sources := make(map[string]ir.Operation, len(document.Operations))
	for _, operation := range document.Operations {
		sources[operation.RouteKey] = operation
	}
	for index := range operations {
		operation := &operations[index]
		if operation.DependencyOnly || operation.Visibility != "" && operation.Visibility != "public" {
			continue
		}
		if duplicateResourceParameter(operation.Path) {
			continue
		}
		source := sources[operation.Route]
		node := root
		supported := true
		for _, part := range httpplan.ResourcePathParts(operation.Path) {
			name, parameter, ok := httpplan.ResourcePathPart(part)
			if !ok {
				supported = false
				break
			}
			prefix := strings.TrimSuffix(node.identity, "/")
			if parameter {
				var contract ir.Parameter
				found := false
				for _, candidate := range source.Parameters {
					if candidate.Location == "path" && candidate.Name == name {
						contract = candidate
						found = true
						break
					}
				}
				if !found || node.parameterBlocked {
					supported = false
					break
				}
				signature := resourcePrimitiveArgument(document, contract.Schema)
				if signature == "" {
					signature = goTypeSignature(document, contract.Schema, true)
				}
				if node.parameter != nil && node.parameterSignature != signature {
					node.parameter = nil
					node.selector = nil
					node.parameterBlocked = true
					supported = false
					break
				}
				if node.parameter == nil {
					node.parameter = &resourceTree{identity: prefix + "/{}", depth: node.depth + 1, children: make(map[string]*resourceTree)}
					node.parameterSignature = signature
					node.selector = &resourceSelectorPlan{Name: "By" + resourceParameterName(name), Key: strconv.Itoa(node.depth), Parameter: contract}
				}
				node = node.parameter
			} else {
				if node.children[part] == nil {
					node.children[part] = &resourceTree{identity: prefix + "/" + part, depth: node.depth, children: make(map[string]*resourceTree)}
				}
				node = node.children[part]
			}
		}
		if supported {
			node.operations = append(node.operations, operation)
		}
	}
	var result []resourceNodePlan
	var visit func(*resourceTree) (bool, error)
	visit = func(node *resourceTree) (bool, error) {
		memberNames := names.resourceMembers[node.identity]
		planned := resourceNodePlan{Name: names.resources[node.identity], Identity: node.identity, Bound: node.depth > 0}
		for _, literal := range sortedKeys(node.children) {
			child := node.children[literal]
			present, err := visit(child)
			if err != nil {
				return false, err
			}
			if !present {
				continue
			}
			name := memberNames["child:"+literal]
			planned.Children = append(planned.Children, resourceMemberPlan{Name: name, Target: names.resources[child.identity]})
		}
		if node.parameter != nil {
			present, err := visit(node.parameter)
			if err != nil {
				return false, err
			}
			if present {
				selector := node.selector
				selector.Child = names.resources[node.parameter.identity]
				selector.Name = memberNames["selector"]
				selector.Argument = resourcePrimitiveArgument(document, selector.Parameter.Schema)
				if selector.Argument == "" {
					selector.Model = names.resourceParameters[node.parameter.identity]
					selector.Validation, err = schemas.prepare(httpValidationName(selector.Model), httpValueSchema(selector.Parameter.Schema), schemaplan.Input, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: selector.Parameter.Pointer + "/schema", MediaType: selector.Parameter.ContentType})
					if err != nil {
						return false, err
					}
					selector.Argument = "models." + selector.Model
				}
				planned.Selector = selector
			}
		}
		for _, operation := range node.operations {
			method := memberNames["call:"+operation.Route]
			operation.ResourceInputName = operation.Name + "Input"
			operation.ResourceHasInput = operation.HasInput
			operation.ResourceRequiredInput = operation.RequiredInput
			if node.depth > 0 {
				operation.ResourceBound = true
				operation.ResourceInputName = operation.Name + "ResourceInput"
				operation.ResourceHasInput = false
				operation.ResourceRequiredInput = false
				for fieldIndex := range operation.InputFields {
					field := &operation.InputFields[fieldIndex]
					if field.Name == "Path" {
						operation.ResourcePathField = field
						continue
					}
					operation.ResourceInputFields = append(operation.ResourceInputFields, *field)
					operation.ResourceHasInput = true
					operation.ResourceRequiredInput = operation.ResourceRequiredInput || field.Required
				}
				for _, name := range sources[operation.Route].PathParameterOrder {
					operation.ResourceBindings = append(operation.ResourceBindings, resourceBindingPlan{Name: name, Key: strconv.Itoa(len(operation.ResourceBindings))})
				}
			}
			planned.Calls = append(planned.Calls, resourceCallPlan{Method: method, Operation: *operation})
		}
		if len(planned.Children) == 0 && planned.Selector == nil && len(planned.Calls) == 0 {
			return false, nil
		}
		result = append(result, planned)
		return true, nil
	}
	if _, err := visit(root); err != nil {
		return nil, err
	}
	return result, nil
}

func resourcePrimitiveArgument(document *ir.Document, value any) string {
	schema, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	planner := extension.Planner{}
	for _, kind := range []struct {
		allowed  map[string]bool
		argument string
	}{
		{map[string]bool{"string": true}, "string"},
		{map[string]bool{"number": true, "integer": true}, "float64"},
		{map[string]bool{"boolean": true}, "bool"},
	} {
		if planner.SchemaHasOnlyTypes(document, schema, kind.allowed) {
			return kind.argument
		}
	}
	return ""
}
