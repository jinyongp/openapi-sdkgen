package golang

import (
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type requestBodyShape struct {
	Media                          ir.MediaType
	Name, ItemSuffix, StreamSuffix string
	Incremental, Sequential        bool
}

// Names and lowering consume the same variants. A complete and incremental
// representation never depend on inspecting the host's value at runtime.
func requestBodyShapes(operation ir.Operation, legacy bool) ([]requestBodyShape, bool, error) {
	body := operation.RequestBody
	if body == nil || len(body.Content) == 0 {
		return nil, false, nil
	}
	union := len(body.Content) > 1
	for _, media := range body.Content {
		_, item := media.Raw["itemSchema"]
		_, schema := media.Raw["schema"]
		union = union || strings.Contains(media.ContentType, "*") || media.Stream.IsStreaming() && item && schema
	}
	mediaNames, variantNames := make(map[string]bool), make(map[string]bool)
	var result []requestBodyShape
	for _, media := range body.Content {
		_, item := media.Raw["itemSchema"]
		_, schema := media.Raw["schema"]
		sequential := media.Stream.IsStreaming() && (item || schema)
		identity := body.Pointer + "/content/" + pointerToken(media.ContentType)
		base := ""
		if union && (len(body.Content) > 1 || !(item && schema)) {
			var err error
			base, err = bodyVariantName(media.ContentType, bodyMediaKind(media.ContentType, media.Schema, legacy), identity, mediaNames)
			if err != nil {
				return nil, false, err
			}
		}
		itemSuffix, streamSuffix := "RequestItem", "RequestStream"
		if len(body.Content) > 1 {
			itemSuffix = base + itemSuffix
			streamSuffix = base + streamSuffix
		}
		appendShape := func(raw, mode string, incremental bool) error {
			name := raw
			if union {
				var err error
				name, err = reservePublicName(variantNames, raw, identity+"/"+mode, "")
				if err != nil {
					return err
				}
			}
			result = append(result, requestBodyShape{Media: media, Name: name, ItemSuffix: itemSuffix, StreamSuffix: streamSuffix, Incremental: incremental, Sequential: sequential})
			return nil
		}
		if sequential && item && schema {
			if err := appendShape(base+"Complete", "complete", false); err != nil {
				return nil, false, err
			}
			if err := appendShape(base+"Stream", "stream", true); err != nil {
				return nil, false, err
			}
		} else if err := appendShape(base, "body", sequential && item); err != nil {
			return nil, false, err
		}
	}
	return result, union, nil
}

func prepareRequestBodyInput(planned *operationPlan, operation ir.Operation, schemas *schemaPlanner, legacy bool, document *ir.Document, nativeInput ...bool) error {
	shapes, union, err := requestBodyShapes(operation, legacy)
	if err != nil {
		return err
	}
	if len(shapes) == 0 {
		return nil
	}
	body := operation.RequestBody
	planned.BodyUnion = union
	streams := make(map[string]*requestStreamPlan)
	for _, shape := range shapes {
		media := shape.Media
		identity := body.Pointer + "/content/" + pointerToken(media.ContentType)
		if strings.TrimSpace(media.ContentType) == "" {
			return fmt.Errorf("request body content requires a non-empty media type or media range at %s", identity)
		}
		multipart := strings.HasPrefix(strings.ToLower(media.ContentType), "multipart/")
		var stream *requestStreamPlan
		if shape.Sequential {
			stream = streams[media.ContentType]
			if stream == nil {
				stream = &requestStreamPlan{Name: planned.Name + shape.StreamSuffix, MediaType: media.ContentType, Framing: string(media.Stream.Framing), Multipart: multipart}
				streams[media.ContentType] = stream
				planned.RequestStreams = append(planned.RequestStreams, stream)
			}
			if !shape.Incremental {
				stream.Complete = true
			}
			if shape.Incremental && stream.ItemValidation == "" {
				stream.ItemModel = planned.Name + shape.ItemSuffix
				item := httpplan.MultipartItemInputSchema(document, media.ContentType, media.ItemSchema, media.Raw["itemEncoding"])
				itemMedia := ""
				if multipart {
					itemMedia = media.ContentType
				}
				validation, err := schemas.prepare(httpValidationName(stream.ItemModel), item, schemaplan.Input, legacy, schemaOccurrence{Source: identity + "/itemSchema", MediaType: itemMedia})
				if err != nil {
					return err
				}
				stream.ItemValidation = validation
			}
		}
		kind := bodyMediaKind(media.ContentType, media.Schema, legacy)
		binary := kind == "binary" && !shape.Sequential
		model := planned.Name + "Body"
		if union {
			model = planned.Name + shape.Name + "Body"
		}
		validation := ""
		if !binary && !shape.Incremental {
			native := len(nativeInput) > 0 && nativeInput[0] && kind == "open" && mediaMayContainMultipart(media.ContentType)
			projectionMedia := media.ContentType
			if native {
				projectionMedia = "multipart/form-data"
			}
			value := httpplan.MultipartInputSchema(document, projectionMedia, media.Schema, media.Raw, true)
			rootMedia := media.ContentType
			if shape.Sequential && !multipart {
				rootMedia = ""
			}
			validation, err = schemas.prepare(httpValidationName(model), value, schemaplan.Input, legacy, schemaOccurrence{Source: identity + "/schema", MediaType: rootMedia})
			if err != nil {
				return err
			}
			if native {
				if schemas.nativeMedia == nil {
					schemas.nativeMedia = make(map[string]bool)
				}
				schemas.nativeMedia[validation] = true
			}
		}
		planned.HasMultipart = planned.HasMultipart || multipart || kind == "open" && mediaMayContainMultipart(media.ContentType)
		if union {
			planned.RequestBodies = append(planned.RequestBodies, bodyVariantPlan{Name: shape.Name, Model: model, MediaType: media.ContentType, Validation: validation, Binary: binary, Multipart: multipart && !shape.Incremental, Wildcard: strings.Contains(media.ContentType, "*"), Open: kind == "open" && !shape.Sequential, BinaryFallback: binaryProjection(media.ContentType, media.Schema, legacy), Index: len(planned.RequestBodies) + 1, Stream: stream, Incremental: shape.Incremental})
		} else {
			planned.InputFields = append(planned.InputFields, inputFieldPlan{Name: "Body", Type: model, Validation: validation, MediaType: media.ContentType, Required: body.Required, Binary: binary, Multipart: multipart, Stream: stream, Incremental: shape.Incremental})
		}
	}
	if union {
		planned.InputFields = append(planned.InputFields, inputFieldPlan{Name: "Body", Type: planned.Name + "Body", Required: body.Required, Union: true})
	}
	planned.RequiredInput = planned.RequiredInput || body.Required
	return nil
}
