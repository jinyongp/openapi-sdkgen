package typescript

import (
	"fmt"
	"strings"
)

type wirePropertiesMode uint8

const (
	wirePropertiesLiteral wirePropertiesMode = iota
	wirePropertiesConstructed
)

// wireRenderContext belongs to one emission owner (or one prepared server
// definition). Nested schemas and media/encoding/header descriptors share it so
// dependency needs come from actual rendering, never from scanning source text.
// The context is not shared across independent generation or emission calls.
type wireRenderContext struct {
	componentNames map[projection]map[string]bool
	schemaPrograms *schemaRuntimePlan
	programImports map[string]schemaProgramImport
	properties     wirePropertiesMode
	usesProperties bool
	semanticOnly   bool
	// Optional semantic observer used by execution planning, never a text parser.
	execution *executionSchemaFacts
}

func newWireRenderContext(mode wirePropertiesMode) *wireRenderContext {
	return &wireRenderContext{properties: mode}
}

// propertyExpression receives the renderer's single sorted, projection-filtered
// list. Exact keys are inert literals and child expressions retain their order.
func (wire *wireRenderContext) propertyExpression(properties []runtimeProperty) (string, error) {
	var output strings.Builder
	switch wire.properties {
	case wirePropertiesLiteral:
		output.WriteString("/* @__PURE__ */ Object.fromEntries<WireProperty>([")
		for index, property := range properties {
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteByte('[')
			output.WriteString(quoteTS(property.key))
			output.WriteString(", { property: ")
			output.WriteString(quoteTS(property.key))
			output.WriteString(", schema: ")
			output.WriteString(property.value)
			output.WriteString(" }]")
		}
	case wirePropertiesConstructed:
		output.WriteString("/* @__PURE__ */ __sdkgen_Properties([")
		for index, property := range properties {
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteString(quoteTS(property.key))
		}
		output.WriteString("], [")
		for index, property := range properties {
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteString(property.value)
		}
	default:
		return "", fmt.Errorf("unsupported wire property construction mode %d", wire.properties)
	}
	output.WriteString("])")
	wire.usesProperties = true
	return output.String(), nil
}
