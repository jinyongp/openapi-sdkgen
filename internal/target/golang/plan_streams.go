package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func prepareOperationStreams(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner, legacy bool, nativeOutput ...bool) error {
	count := 0
	parts := newResponsePartPlanner(planned, document, schemas)
	for index := range planned.Responses {
		branch := &planned.Responses[index]
		if !branch.Sequential {
			continue
		}
		for _, response := range operation.Responses {
			if response.Status != branch.Status {
				continue
			}
			for _, media := range response.Content {
				if media.ContentType != branch.MediaType {
					continue
				}
				_, declaredItem := media.Raw["itemSchema"]
				branch.TypedStream = declaredItem && branch.StreamSuccess
				if !declaredItem && !branch.Buffered {
					continue
				}
				item := media.ItemSchema
				if !declaredItem {
					if schema, ok := media.Schema.(map[string]any); ok {
						item = schema["items"]
					}
				}
				if item == nil {
					item = true
				}
				branch.Framing = string(media.Stream.Framing)
				if branch.Framing == "multipart" {
					planned.HasMultipartStream = true
					var err error
					branch.CompleteParts, err = parts.prepare(media.Schema, media.Raw)
					if err != nil {
						return err
					}
					branch.ItemParts, err = parts.prepare(map[string]any{"items": media.ItemSchema}, map[string]any{"itemEncoding": media.Raw["itemEncoding"]})
					if err != nil {
						return err
					}
					if branch.Buffered && branch.Success {
						planned.OutputUnion = true
						planned.OutputType = planned.Name + "Output"
					}
				}
				branch.ItemModel = planned.Name + branch.Name + "Item"
				itemMedia := ""
				native := len(nativeOutput) > 0 && nativeOutput[0] && branch.Framing == "multipart"
				if native {
					item = httpplan.MultipartItemInputSchema(document, media.ContentType, item, media.Raw["itemEncoding"])
					itemMedia = media.ContentType
				}
				validation, err := schemas.prepare(httpValidationName(branch.ItemModel), item, schemaplan.Output, legacy, schemaOccurrence{Source: response.Pointer + "/content/" + pointerToken(media.ContentType) + "/itemSchema", MediaType: itemMedia})
				if err != nil {
					return err
				}
				branch.ItemValidation = validation
				if native {
					if schemas.nativeMedia == nil {
						schemas.nativeMedia = make(map[string]bool)
					}
					schemas.nativeMedia[validation] = true
				}
				if branch.TypedStream {
					count++
					planned.HasTypedStream = true
					planned.StreamType = "models." + branch.ItemModel
					if planned.StreamAccept != "" {
						planned.StreamAccept += ", "
					}
					planned.StreamAccept += branch.MediaType
				}
			}
		}
	}
	planned.BodyEncodings += parts.source()
	planned.StreamUnion = count > 1 || planned.HasTypedStream && planned.HasMultipartStream
	if planned.StreamUnion {
		planned.StreamType = planned.Name + "StreamItem"
	}
	return nil
}
