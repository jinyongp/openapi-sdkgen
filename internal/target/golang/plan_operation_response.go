package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/http/extension"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strings"
)

func responseMediaKind(media string) string {
	bare := bareMediaType(media)
	if bare == "" {
		return "none"
	}
	if bare == "application/json" || strings.HasSuffix(bare, "+json") {
		return "json"
	}
	if isXMLMedia(media) {
		return "xml"
	}
	if strings.HasPrefix(bare, "text/") && bare != "text/event-stream" {
		return "text"
	}
	if bare == "application/octet-stream" || strings.HasPrefix(bare, "image/") || strings.HasPrefix(bare, "audio/") || strings.HasPrefix(bare, "video/") {
		return "binary"
	}
	if strings.Contains(bare, "*") {
		return "open"
	}
	return "custom"
}
func bodyMediaKind(media string, schema any, legacy bool) string {
	kind := responseMediaKind(media)
	if schema == false {
		return "blocked"
	}
	if kind == "open" {
		return kind
	}
	if kind != "json" && kind != "text" && binaryProjection(media, schema, legacy) {
		return "binary"
	}
	return kind
}
func binaryProjection(media string, schema any, legacy bool) bool {
	version := "3.1"
	if legacy {
		version = "3.0"
	}
	object, _ := schema.(map[string]any)
	return schema != false && httpplan.IsBinaryMedia(version, bareMediaType(media), object)
}

func prepareOperationResponses(planned *operationPlan, operation ir.Operation, schemas *schemaPlanner, openAPI30 bool, document *ir.Document, nativeOutput ...bool) error {
	successes := make(map[string]bool)
	for _, branch := range httpplan.Branches(operation.Responses, true) {
		media := ""
		if branch.Media != nil {
			media = branch.Media.ContentType
		}
		successes[branch.Response.Status+"\x00"+media] = true
	}
	count := 0
	used := make(map[string]bool)
	for _, response := range operation.Responses {
		content := response.Content
		if len(content) == 0 {
			content = []ir.MediaType{{}}
		}
		for _, media := range content {
			kind := bodyMediaKind(media.ContentType, media.Schema, openAPI30)
			sequential := media.Stream.Framing != "" && media.Stream.Framing != ir.StreamFramingNone
			_, completeSchema := media.Raw["schema"]
			buffered := sequential && completeSchema
			if sequential {
				kind = "stream"
				planned.HasSequential = true
			}
			name, err := responseVariantName(operation, response, media, kind, used)
			if err != nil {
				return err
			}
			branch := responsePlan{Status: response.Status, MediaType: media.ContentType, Kind: kind, Name: name, Index: len(planned.Responses) + 1, Success: successes[response.Status+"\x00"+media.ContentType]}
			branch.Open = kind == "open"
			branch.BinaryFallback = branch.Open && binaryProjection(media.ContentType, media.Schema, openAPI30)
			planned.HasOpenMedia = planned.HasOpenMedia || branch.Open
			branch.Sequential = sequential
			branch.Buffered = buffered
			branch.StreamSuccess = branch.Success
			if branch.Success {
				if !sequential || buffered {
					count++
				}
			}
			if sequential && !buffered {
				branch.Success = false
			}
			branch.Model = planned.Name + name
			planned.Responses = append(planned.Responses, branch)
		}
	}
	planned.OutputUnion = count > 1
	planned.OutputType = "types.NoContent"
	for index := range planned.Responses {
		branch := &planned.Responses[index]
		if branch.Success && count == 1 && planned.Envelope != "data" {
			branch.Model = planned.Name + "Output"
		}
		switch branch.Kind {
		case "none":
			branch.Type = "types.NoContent"
		case "binary":
			branch.Type = "[]byte"
		case "stream":
			if !branch.Buffered {
				branch.Type = "types.NoContent"
				break
			}
			fallthrough
		default:
			branch.Type = "models." + branch.Model
			if branch.Open {
				branch.Type = "types.MediaValue[models." + branch.Model + "]"
			}
			var schema any
			var mediaRaw map[string]any
			var source string
			for _, response := range operation.Responses {
				if response.Status == branch.Status {
					source = response.Pointer
					for _, media := range response.Content {
						if media.ContentType == branch.MediaType {
							schema = media.Schema
							mediaRaw = media.Raw
						}
					}
				}
			}
			native := len(nativeOutput) > 0 && nativeOutput[0] && mediaMayContainMultipart(branch.MediaType)
			if native {
				projectionMedia := branch.MediaType
				if branch.Open {
					projectionMedia = "multipart/form-data"
				}
				schema = httpplan.MultipartInputSchema(document, projectionMedia, schema, mediaRaw, true)
			}
			validation, err := schemas.prepare(httpValidationName(branch.Model), schema, schemaplan.Output, openAPI30, schemaOccurrence{Source: source + "/content/" + pointerToken(branch.MediaType) + "/schema", MediaType: branch.MediaType})
			if err != nil {
				return err
			}
			branch.Validation = validation
			if native {
				if schemas.nativeMedia == nil {
					schemas.nativeMedia = make(map[string]bool)
				}
				schemas.nativeMedia[validation] = true
			}
			planned.ResponseValidation = true
			if branch.Success && planned.Envelope == "data" {
				object, _ := schema.(map[string]any)
				data := (&extension.Planner{}).EnvelopeSchema(document, object, make(map[string]bool))
				branch.DataModel = planned.Name + branch.Name + "Data"
				if count == 1 {
					branch.DataModel = planned.Name + "Output"
				}
				branch.DataValidation, err = schemas.prepare(httpValidationName(branch.DataModel), data, schemaplan.Output, openAPI30, schemaOccurrence{Source: source + "/content/" + pointerToken(branch.MediaType) + "/schema/properties/data"})
				if err != nil {
					return err
				}
				branch.DataType = "models." + branch.DataModel
				if branch.Open {
					branch.DataType = "types.MediaValue[models." + branch.DataModel + "]"
				}
				branch.EnvelopeFallback = !envelopeDataRequired(document, object, make(map[string]bool))
				planned.OutputUnion = planned.OutputUnion || branch.EnvelopeFallback
			}
		}
		if branch.Success && !planned.OutputUnion {
			planned.OutputType = branch.Type
			if branch.DataType != "" {
				planned.OutputType = branch.DataType
			}
		}
	}
	if planned.OutputUnion {
		planned.OutputType = planned.Name + "Output"
	}
	planned.CanDo = !planned.HasSequential || count > 0
	return nil
}
