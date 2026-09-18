package typescript

import (
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

type generatedStream struct {
	Operation ir.Operation
	ItemType  string
	Plan      ManifestOperation
}

func generatedStreams(document *ir.Document, manifest Manifest) ([]generatedStream, error) {
	result, failures := generatedStreamsDiagnostics(document, manifest)
	if len(failures) != 0 {
		return nil, failures[0]
	}
	return result, nil
}

func generatedStreamsDiagnostics(document *ir.Document, manifest Manifest) ([]generatedStream, []error) {
	visible := map[string]ManifestOperation{}
	for _, operation := range manifest.Operations {
		if operation.Visibility != "hidden" {
			visible[manifestRouteKey(operation)] = operation
		}
	}
	var result []generatedStream
	var failures []error
	for _, operation := range document.Operations {
		operationPlan, operationVisible := visible[operationRouteKey(operation)]
		if !operationVisible {
			continue
		}
		sets, err := operationResponseMediaSets(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("streaming response %s: %w", operationLabel(operation), err))
			continue
		}
		var types []string
		for _, response := range sets.streaming {
			if !isSuccessResponseStatus(response.Status) {
				continue
			}
			for _, media := range response.Content {
				if _, exists := media.Raw["itemSchema"]; !exists {
					failures = append(failures, fmt.Errorf("streaming response %s %s has no itemSchema", operationLabel(operation), media.ContentType))
					continue
				}
				itemType, err := schemaTypeForScope(document, media.ItemSchema, projectionOutput, typeRenderContract)
				if err != nil {
					failures = append(failures, fmt.Errorf("streaming response %s %s item schema: %w", operationLabel(operation), media.ContentType, err))
					continue
				}
				types = append(types, itemType)
			}
		}
		if len(types) != 0 {
			result = append(result, generatedStream{Operation: operation, ItemType: stringsJoinUnique(types, " | "), Plan: operationPlan})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return operationRouteKey(result[left].Operation) < operationRouteKey(result[right].Operation)
	})
	return result, failures
}

func streamForRoute(streams []generatedStream, routeKey string) (generatedStream, bool) {
	for _, stream := range streams {
		if operationRouteKey(stream.Operation) == routeKey {
			return stream, true
		}
	}
	return generatedStream{}, false
}

func streamOptionsType(stream generatedStream) string {
	base := operationSlotType(operationRouteKey(stream.Operation), "options")
	base = "Omit<" + base + ", \"accept\">"
	if len(stream.Plan.streamMediaTypes) <= 1 {
		return base
	}
	accepted := make([]string, 0, len(stream.Plan.streamMediaTypes))
	for _, mediaType := range stream.Plan.streamMediaTypes {
		accepted = append(accepted, quoteTS(mediaType))
	}
	return base + " & { readonly accept?: " + strings.Join(accepted, " | ") + " | undefined }"
}

func streamFunctionType(document *ir.Document, stream generatedStream) (string, error) {
	_ = document
	inputType := operationSlotType(operationRouteKey(stream.Operation), "input")
	return streamFunctionTypeForInput(stream, inputType, len(stream.Plan.InputTypes) > 0, stream.Plan.prepared.inputRequired), nil
}

func resourceStreamFunctionType(document *ir.Document, stream generatedStream) (string, error) {
	_ = document
	if len(stream.Plan.PathParameterOrder) == 0 {
		return streamFunctionType(document, stream)
	}
	hasInput := len(stream.Plan.InputTypes) > 1
	inputType := operationSlotType(operationRouteKey(stream.Operation), "resourceInput")
	return streamFunctionTypeForInput(stream, inputType, hasInput, stream.Plan.prepared.resourceInputRequired), nil
}

func streamFunctionTypeForInput(stream generatedStream, inputType string, hasInput, inputRequired bool) string {
	optionsType := streamOptionsType(stream)
	optionMarker := "?"
	if stream.Plan.optionsRequired {
		optionMarker = ""
	}
	if !hasInput {
		return "(options" + optionMarker + ": " + optionsType + ") => OperationStream<" + stream.ItemType + ">"
	}
	if !inputRequired {
		optionsOnly := "(options" + optionMarker + ": " + optionsType + ") => OperationStream<" + stream.ItemType + ">"
		inputMarker := ""
		if optionMarker == "?" {
			inputMarker = "?"
		} else {
			inputType += " | undefined"
		}
		inputCall := "(input" + inputMarker + ": " + inputType + ", options" + optionMarker + ": " + optionsType + ") => OperationStream<" + stream.ItemType + ">"
		return "(" + optionsOnly + ") & (" + inputCall + ")"
	}
	return "(input: " + inputType + ", options" + optionMarker + ": " + optionsType + ") => OperationStream<" + stream.ItemType + ">"
}

func operationRequiresOptions(document *ir.Document, operation ir.Operation) (bool, error) {
	return operationRequiresSecuritySelection(document, operation)
}

func isStreamMediaType(mediaType string) bool {
	return ir.StreamPlanForMediaType(mediaType, false).IsStreaming()
}

func stringsJoinUnique(values []string, separator string) string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return strings.Join(result, separator)
}
