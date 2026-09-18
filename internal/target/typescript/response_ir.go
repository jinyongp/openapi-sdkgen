package typescript

import (
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func operationResponses(document *ir.Document, operation ir.Operation) ([]ir.Response, error) {
	if operation.Responses != nil {
		return operation.Responses, nil
	}
	return syntheticOperationResponses(document, operation)
}

type responseMediaSets struct {
	normal    []ir.Response
	streaming []ir.Response
}

func operationResponseMediaSets(document *ir.Document, operation ir.Operation) (responseMediaSets, error) {
	responses, err := operationResponses(document, operation)
	if err != nil {
		return responseMediaSets{}, err
	}
	sets := responseMediaSets{
		normal:    make([]ir.Response, 0, len(responses)),
		streaming: make([]ir.Response, 0, len(responses)),
	}
	for _, response := range responses {
		if len(response.Content) == 0 {
			sets.normal = append(sets.normal, response)
			continue
		}
		normalContent := make([]ir.MediaType, 0, len(response.Content))
		streamContent := make([]ir.MediaType, 0, len(response.Content))
		for _, media := range response.Content {
			if media.Stream.IsStreaming() {
				streamContent = append(streamContent, media)
			} else {
				normalContent = append(normalContent, media)
			}
		}
		if len(normalContent) != 0 {
			normalResponse := response
			normalResponse.Content = normalContent
			sets.normal = append(sets.normal, normalResponse)
		}
		if len(streamContent) != 0 {
			streamResponse := response
			streamResponse.Content = streamContent
			sets.streaming = append(sets.streaming, streamResponse)
		}
	}
	return sets, nil
}

// syntheticOperationResponses preserves the raw-map path used by focused
// target tests that construct ir.Operation directly instead of compiling an
// OpenAPI document. Compiler-built path operations always carry a non-nil
// Responses slice.
func syntheticOperationResponses(document *ir.Document, operation ir.Operation) ([]ir.Response, error) {
	values, _ := operation.Raw["responses"].(map[string]any)
	statuses := make([]string, 0, len(values))
	for status := range values {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	result := make([]ir.Response, 0, len(statuses))
	for _, status := range statuses {
		if strings.HasPrefix(status, "x-") {
			continue
		}
		source, _ := values[status].(map[string]any)
		resolved, err := resolveComponentObject(document, source, "responses")
		if err != nil {
			return nil, err
		}
		content, _ := resolved["content"].(map[string]any)
		mediaTypes := make([]string, 0, len(content))
		for mediaType := range content {
			mediaTypes = append(mediaTypes, mediaType)
		}
		sort.Strings(mediaTypes)
		media := make([]ir.MediaType, 0, len(mediaTypes))
		for _, contentType := range mediaTypes {
			value, _ := content[contentType].(map[string]any)
			value, err = resolveMediaTypeObject(document, value)
			if err != nil {
				return nil, err
			}
			_, hasItemSchema := value["itemSchema"]
			media = append(media, ir.MediaType{
				ContentType: contentType,
				Schema:      value["schema"],
				ItemSchema:  value["itemSchema"],
				Stream:      ir.StreamPlanForMediaType(contentType, hasItemSchema),
				Raw:         value,
			})
		}
		description, _ := resolved["description"].(string)
		summary, _ := resolved["summary"].(string)
		result = append(result, ir.Response{
			Status:      status,
			Description: description,
			Summary:     summary,
			Content:     media,
			Raw:         resolved,
			SourceRaw:   source,
			Pointer:     responsePointer(operation, status),
		})
	}
	return result, nil
}
