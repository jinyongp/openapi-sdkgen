package golang

import (
	"encoding/json"
	"openapi-sdkgen/internal/compiler/ir"
	"strings"
	"unicode"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *modelPlanner) promoteAnnotatedEnum(node *schemaplan.Node) (*schemaplan.Node, error) {
	if node.Get("enumValues") != nil || node.Get("constValue") != nil {
		return node, nil
	}
	source, _ := planner.annotations[node].(map[string]any)
	members, recognized, err := schemaplan.AnnotatedEnum(planner.version, source, func(value any) (string, error) { encoded, err := json.Marshal(value); return string(encoded), err })
	if err != nil || !recognized {
		return node, err
	}
	representation := &schemaplan.Node{}
	for _, field := range node.Fields {
		if field.Name != "oneOf" && field.Name != "anyOf" {
			representation.Fields = append(representation.Fields, field)
		}
	}
	values := make([]any, 0, len(members))
	docs := make(map[string]schemaplan.Documentation, len(members))
	for _, member := range members {
		values = append(values, member.Value)
		encoded, _ := json.Marshal(member.Value)
		docs[string(encoded)] = member.Documentation
	}
	representation.Fields = append(representation.Fields, schemaplan.Field{Name: "enumValues", Value: schemaplan.Literal{Data: values}})
	if planner.memberDocs == nil {
		planner.memberDocs = make(map[*schemaplan.Node]map[string]schemaplan.Documentation)
	}
	planner.memberDocs[representation] = docs
	planner.annotations[representation] = planner.annotations[node]
	planner.validators[representation] = planner.validators[node]
	planner.sources[representation] = planner.sources[node]
	return representation, nil
}

func (planner *modelPlanner) documentation(node *schemaplan.Node) string {
	source, _ := planner.annotations[node].(map[string]any)
	documentation := schemaplan.SchemaDocumentation(source)
	var annotations []string
	for _, key := range append([]string{"$schema", "$comment", "format", "default", "example", "examples", "externalDocs", "readOnly", "writeOnly"}, schemaplan.ConstraintKeywords()...) {
		if value, present := source[key]; present {
			if encoded, err := json.Marshal(value); err == nil {
				annotations = append(annotations, key+"="+string(encoded))
			}
		}
	}
	if len(annotations) != 0 {
		if documentation.Description != "" {
			documentation.Description += "\n\n"
		}
		documentation.Description += "OpenAPI: " + strings.Join(annotations, ", ")
	}
	documentation.Deprecated = schemaplan.AlwaysDeprecated(source, func(reference string) any {
		name, err := schemaplan.ComponentReferenceName(reference)
		if err != nil {
			return nil
		}
		return planner.componentSources[name]
	})
	return goDocumentation(documentation)
}

func operationDocumentation(operation ir.Operation) string {
	deprecated, _ := operation.Raw["deprecated"].(bool)
	return goDocumentation(schemaplan.Documentation{Title: operation.Summary, Description: operation.Description, Deprecated: deprecated})
}

func goDocumentation(documentation schemaplan.Documentation) string {
	parts := make([]string, 0, 3)
	if documentation.Title != "" {
		parts = append(parts, documentation.Title)
	}
	if documentation.Description != "" {
		parts = append(parts, documentation.Description)
	}
	if documentation.Deprecated {
		parts = append(parts, "Deprecated: This OpenAPI definition is deprecated.")
	}
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, "\n\n")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) && r != '\ufeff' {
			return r
		}
		return ' '
	}, text)
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = "// " + line
	}
	return strings.Join(lines, "\n") + "\n"
}
