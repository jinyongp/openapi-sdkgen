package emit

import (
	"encoding/json"
	"openapi-sdkgen/internal/target/typescript/schema/plan"
)

// Reference dispatch reads its target from the owning WireSchema. The target
// name belongs to that descriptor, while the execution algorithm is shared.
// Other semantic fields retain their complete identity conservatively.
func ProgramIdentity(node *plan.Node) string {
	data, err := json.Marshal(programIdentityNode(node))
	if err != nil {
		panic(err)
	} // Lower has already validated all literal data.
	return string(data)
}

func programIdentityNode(node *plan.Node) *plan.Node {
	result := &plan.Node{Fields: make([]plan.Field, len(node.Fields))}
	for index, field := range node.Fields {
		value := field.Value
		if field.Name == "reference" {
			value = plan.Literal{Data: ""}
		} else {
			switch typed := value.(type) {
			case plan.Child:
				value = plan.Child{Node: programIdentityNode(typed.Node)}
			case plan.Children:
				children := make([]*plan.Node, len(typed.Nodes))
				for i, child := range typed.Nodes {
					children[i] = programIdentityNode(child)
				}
				value = plan.Children{Nodes: children}
			case plan.Properties:
				value = plan.Properties{Entries: programIdentityProperties(typed.Entries)}
			case plan.SchemaMap:
				value = plan.SchemaMap{Entries: programIdentityProperties(typed.Entries)}
			case plan.DynamicReference:
				value = plan.DynamicReference{Anchor: typed.Anchor, Fallback: programIdentityNode(typed.Fallback)}
			case plan.Discriminator:
				typed.Mapping = programIdentityProperties(typed.Mapping)
				if typed.Default != nil {
					typed.Default = programIdentityNode(typed.Default)
				}
				value = typed
			}
		}
		result.Fields[index] = plan.Field{Name: field.Name, Value: value}
	}
	return result
}

func programIdentityProperties(properties []plan.Property) []plan.Property {
	result := make([]plan.Property, len(properties))
	for index, property := range properties {
		result[index] = plan.Property{Name: property.Name, Schema: programIdentityNode(property.Schema)}
	}
	return result
}
