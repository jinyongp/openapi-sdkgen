package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"openapi-sdkgen/internal/compiler/ir"
	"strings"
)

func httpValidationName(root string) string { return "ValidateHTTP__" + root + "__" }
func headerValidationName(root, identity string) string {
	digest := sha256.Sum256([]byte(root + "\x00" + identity))
	return "ValidateHeader__" + hex.EncodeToString(digest[:]) + "__"
}

func bodyVariantName(mediaType, kind, identity string, used map[string]bool) (string, error) {
	suffix := map[string]string{"json": "JSON", "xml": "XML", "text": "Text", "binary": "Binary", "open": "Media"}[kind]
	if suffix == "" {
		suffix = exportedName(mediaType)
	}
	return reservePublicName(used, suffix, identity, "")
}
func responseVariantName(operation ir.Operation, response ir.Response, media ir.MediaType, kind string, used map[string]bool) (string, error) {
	status := strings.ToUpper(response.Status)
	if response.Status == "default" {
		status = "Default"
	}
	suffix := map[string]string{"none": "NoContent", "json": "JSON", "xml": "XML", "text": "Text", "binary": "Binary", "stream": "Stream", "open": "Media"}[kind]
	if suffix == "" {
		suffix = exportedName(media.ContentType)
	}
	origin := response.Pointer
	if origin == "" {
		origin = operation.Pointer + "/responses/" + response.Status
	}
	reserved, err := reservePublicName(used, "Response"+status+suffix, origin+"/content/"+pointerToken(media.ContentType), "")
	return strings.TrimPrefix(reserved, "Response"), err
}

// Reserve every public contract root before lowering nested DTOs or selecting
// operations. Otherwise a nested component model can occupy a later HTTP root.
func operationContractNames(operation ir.Operation, legacy bool) (roots, factories []string, err error) {
	roots = append(roots, "Operation", "ResourceInput", "ResourcePath", "PaginationItem", "PaginationMode", "LinkSource", "Links", "LinkOverride", "LinkOptions")
	for _, section := range []string{"Path", "Query", "Header", "Cookie", "Querystring", "Body"} {
		roots = append(roots, "LinkOverride"+section)
	}
	shapes, union, shapeError := requestBodyShapes(operation, legacy)
	if shapeError != nil {
		return nil, nil, shapeError
	}
	for _, shape := range shapes {
		if union {
			roots = append(roots, shape.Name+"Body")
			factories = append(factories, shape.Name+"Body")
			if !shape.Sequential && bodyMediaKind(shape.Media.ContentType, shape.Media.Schema, legacy) == "open" {
				factories = append(factories, shape.Name+"BinaryBody")
			}
		}
		if shape.Incremental {
			roots = append(roots, shape.ItemSuffix)
		}
	}
	used := make(map[string]bool)
	for _, response := range operation.Responses {
		status := strings.ToUpper(response.Status)
		if response.Status == "default" {
			status = "Default"
		}
		roots = append(roots, status+"Headers")
		content := response.Content
		if len(content) == 0 {
			content = []ir.MediaType{{}}
		}
		for _, media := range content {
			kind := bodyMediaKind(media.ContentType, media.Schema, legacy)
			if media.Stream.Framing != "" && media.Stream.Framing != ir.StreamFramingNone {
				kind = "stream"
			}
			name, nameErr := responseVariantName(operation, response, media, kind, used)
			if nameErr != nil {
				return nil, nil, nameErr
			}
			roots = append(roots, name)
			if operation.Envelope == "data" {
				roots = append(roots, name+"Data")
			}
			if media.Stream.IsStreaming() {
				roots = append(roots, name+"Item", "StreamItem")
			}
		}
	}
	return roots, factories, nil
}
