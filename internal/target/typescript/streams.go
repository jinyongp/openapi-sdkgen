package typescript

import (
	"bytes"
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

func emitStreamInterface(output *bytes.Buffer, document *ir.Document, streams []generatedStream) error {
	if len(streams) == 0 {
		return nil
	}
	output.WriteString("  /** Lazy typed response streams keyed by OpenAPI operation ID. */\n")
	output.WriteString("  readonly $streams: {\n")
	for _, stream := range streams {
		if stream.Operation.OperationID == "" {
			continue
		}
		fmt.Fprintf(output, "    readonly %s: StreamCall<%s>\n", quoteTS(stream.Operation.OperationID), quoteTS(operationRouteKey(stream.Operation)))
	}
	output.WriteString("  }\n")
	return nil
}

func emitStreamValues(output *bytes.Buffer, document *ir.Document, streams []generatedStream) error {
	for _, stream := range streams {
		definition, err := operationDefinition(document, stream.Operation, stream.Plan)
		if err != nil {
			return err
		}
		inputs := stream.Plan.InputTypes
		inputRequired := stream.Plan.prepared.inputRequired
		inputType := operationSlotType(operationRouteKey(stream.Operation), "input")
		optionsType := streamOptionsType(stream)
		variable := stablePrivateIdentifier("stream-value", operationRouteKey(stream.Operation))
		functionType, err := streamFunctionType(document, stream)
		if err != nil {
			return err
		}
		hasInput := len(inputs) > 0
		inputOptional := hasInput && !inputRequired
		fmt.Fprintf(output, "  const %s = bindStreamOperation<%s, %s, %s>(request, %s, %t, %t) as %s\n", variable, inputType, stream.ItemType, optionsType, definition, hasInput, inputOptional, functionType)
	}
	return nil
}

func emitStreamReturnValue(output *bytes.Buffer, streams []generatedStream) error {
	if len(streams) == 0 {
		return nil
	}
	values := make([]runtimeProperty, 0, len(streams))
	for _, stream := range streams {
		if stream.Operation.OperationID == "" {
			continue
		}
		values = append(values, runtimeProperty{
			key:   stream.Operation.OperationID,
			value: stablePrivateIdentifier("stream-value", operationRouteKey(stream.Operation)),
		})
	}
	fmt.Fprintf(output, "    $streams: %s as unknown as Client[\"$streams\"],\n", runtimeObjectExpression(values))
	return nil
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
	inputs := stream.Plan.InputTypes
	inputRequired := stream.Plan.prepared.inputRequired
	optionsType := streamOptionsType(stream)
	optionMarker := "?"
	optionsRequired := stream.Plan.optionsRequired
	if optionsRequired {
		optionMarker = ""
	}
	if len(inputs) == 0 {
		return "(options" + optionMarker + ": " + optionsType + ") => AsyncIterable<" + stream.ItemType + ">", nil
	}
	inputType := operationSlotType(operationRouteKey(stream.Operation), "input")
	if !inputRequired {
		optionsOnly := "(options" + optionMarker + ": " + optionsType + ") => AsyncIterable<" + stream.ItemType + ">"
		inputMarker := ""
		if optionMarker == "?" {
			inputMarker = "?"
		} else {
			inputType += " | undefined"
		}
		inputCall := "(input" + inputMarker + ": " + inputType + ", options" + optionMarker + ": " + optionsType + ") => AsyncIterable<" + stream.ItemType + ">"
		return "(" + optionsOnly + ") & (" + inputCall + ")", nil
	}
	inputMarker := ""
	return "(input" + inputMarker + ": " + inputType + ", options" + optionMarker + ": " + optionsType + ") => AsyncIterable<" + stream.ItemType + ">", nil
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
