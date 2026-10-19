package golang

import (
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strings"
)

// Reverse HTTP media occurrences into the existing XML encoder/decoder plans.
// These views participate only in codec preparation, never client generation.
func prepareInboundXMLViews(entries []inboundOperationPlan, schemas *schemaPlanner) []operationPlan {
	var result []operationPlan
	for index := range entries {
		entry := &entries[index]
		view := operationPlan{Name: entry.Operation.Name + "InboundXML"}
		request := func(key, model, validation, mediaType string) {
			if (isXMLMedia(mediaType) || strings.Contains(mediaType, "*")) && validation != "" {
				view.Responses = append(view.Responses, responsePlan{Name: key, Model: model, Validation: validation, Kind: "xml", MediaType: mediaType})
			}
		}
		for _, field := range entry.Operation.InputFields {
			request("body:"+field.Name, field.Type, field.Validation, field.MediaType)
		}
		for _, body := range entry.Operation.RequestBodies {
			request("body:"+body.Name, body.Model, body.Validation, body.MediaType)
		}
		for _, parameter := range entry.Operation.Parameters {
			if !isXMLMedia(parameter.ContentType) {
				continue
			}
			for _, section := range entry.Operation.InputFields {
				if section.Name != parameter.Section {
					continue
				}
				properties, _ := schemas.nodes[section.Validation].Get("properties").(schemaplan.Properties)
				for _, property := range properties.Entries {
					if property.Name == parameter.Name {
						binding := schemas.nodeNames[property.Schema]
						schemas.nodes[binding] = property.Schema
						request("parameter:"+parameter.Location+":"+parameter.Name, binding, binding, parameter.ContentType)
					}
				}
			}
		}
		for _, response := range entry.Operation.Responses {
			if (isXMLMedia(response.MediaType) || response.Open) && response.Validation != "" {
				view.InputFields = append(view.InputFields, inputFieldPlan{Name: "response:" + response.Name, Type: response.Model, Validation: response.Validation, MediaType: "application/xml"})
			}
		}
		for _, group := range entry.Operation.Headers {
			for _, header := range group.Fields {
				if isXMLMedia(header.ContentType) {
					view.InputFields = append(view.InputFields, inputFieldPlan{Name: "header:" + group.Status + ":" + header.Name, Type: header.Validation, Validation: header.Validation, MediaType: header.ContentType})
				}
			}
		}
		entry.XMLViews = view
		result = append(result, view)
	}
	return result
}

func inboundXMLDecoder(entry inboundOperationPlan, key string) string {
	for _, response := range entry.XMLViews.Responses {
		if response.Name == key {
			return "schema." + response.Decoder
		}
	}
	return "nil"
}

func inboundXMLEncoder(entry inboundOperationPlan, key string) string {
	for _, field := range entry.XMLViews.InputFields {
		if field.Name == key {
			return "schema." + field.Encoder
		}
	}
	return "nil"
}
