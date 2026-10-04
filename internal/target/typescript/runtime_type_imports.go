package typescript

import "strings"

// Public names retain one canonical contract owner after runtime separation.
func runtimeTypeTemplate(name, fallback string) string {
	switch name {
	case "BasicRequestParameterServices":
		return "http-request-parameter-types.ts"
	case "MediaCodec":
		return "media-codec-types.ts"
	case "StreamReader", "StreamContext", "StreamProtocol", "StreamAdapter", "StreamCodec", "StreamFraming":
		return "stream-protocol-types.ts"
	case "WireBodyDefinition", "WireEncodingDefinition", "WireMultipartHeaderDefinition", "WireHeaderDefinition":
		return "media-contract-types.ts"
	case "WireResponseDefinition":
		return "http-response-types.ts"
	case "HTTPStreamDecodeOptions", "MultipartStreamPart", "EncodedStreamRequestBody", "IncrementalStreamRequestOptions", "CompleteSequentialRequestOptions", "StreamProtocolEncodeOptions", "MultipartPartValue", "HTTPCodecExtensions":
		return "media-service-types.ts"
	default:
		return fallback
	}
}

func emitRuntimeTypeImports(names []string, fallback string, importFrom func(string, string) error) error {
	groups := make(map[string]map[string]bool)
	for _, name := range names {
		template := runtimeTypeTemplate(name, fallback)
		if groups[template] == nil {
			groups[template] = make(map[string]bool)
		}
		groups[template][name] = true
	}
	for _, template := range sortedRuntimeHandlerKeys(groups) {
		if err := importFrom("type { "+strings.Join(sortedStringKeys(groups[template]), ", ")+" }", runtimeTemplatePath(template)); err != nil {
			return err
		}
	}
	return nil
}
