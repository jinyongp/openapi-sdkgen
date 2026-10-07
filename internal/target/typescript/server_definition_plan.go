package typescript

import "openapi-sdkgen/internal/compiler/ir"

// Prepared definitions retain resolved input identity. Runtime expressions are
// rendered on emission-local copies using the complete receiving file's owner.
type inboundDefinitionSource struct {
	pathItem  map[string]any
	operation map[string]any
	path      string
}

type inboundDefinitionRendering struct {
	parameters   string
	body         inboundBodyDefinition
	responsePlan string
}

func renderInboundDefinition(wire *wireRenderContext, document *ir.Document, source inboundDefinitionSource) (inboundDefinitionRendering, error) {
	parameters, _, err := wire.inboundParameterDefinitions(document, source.pathItem, source.operation, source.path, true)
	if err != nil {
		return inboundDefinitionRendering{}, err
	}
	body, err := wire.inboundBodyType(document, source.operation, source.path)
	if err != nil {
		return inboundDefinitionRendering{}, err
	}
	_, response, err := wire.inboundResponseDefinition(document, source.operation, source.path)
	return inboundDefinitionRendering{parameters: parameters, body: body, responsePlan: response}, err
}

func newServerValueIdentifierPlan(owner string) (*localIdentifierPlan, error) {
	fixed, err := newServerIdentifierPlan(owner)
	if err != nil {
		return nil, err
	}
	names := newLocalIdentifierPlan(owner)
	for name := range fixed.reserved {
		if err := names.reserve(name); err != nil {
			return nil, err
		}
	}
	return names, nil
}
