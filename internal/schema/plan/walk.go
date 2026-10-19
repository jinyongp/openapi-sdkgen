package plan

// Walk visits each normalized node once, including dynamic fallbacks and
// discriminator contracts. Callers do not need a second keyword inventory.
func Walk(root *Node, consume func(*Node)) {
	seen := make(map[*Node]bool)
	var visit func(*Node)
	visit = func(node *Node) {
		if node == nil || seen[node] {
			return
		}
		seen[node] = true
		consume(node)
		for _, field := range node.Fields {
			switch value := field.Value.(type) {
			case Child:
				visit(value.Node)
			case Children:
				for _, child := range value.Nodes {
					visit(child)
				}
			case Properties:
				for _, property := range value.Entries {
					visit(property.Schema)
				}
			case SchemaMap:
				for _, property := range value.Entries {
					visit(property.Schema)
				}
			case DynamicReference:
				visit(value.Fallback)
			case Discriminator:
				for _, property := range value.Mapping {
					visit(property.Schema)
				}
				visit(value.Default)
			}
		}
	}
	visit(root)
}
