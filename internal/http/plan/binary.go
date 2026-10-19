package plan

import "strings"

// IsBinaryMedia applies the versioned complete-value projection used by HTTP
// bodies. JSON, text and XML keep their structured/string representations.
func IsBinaryMedia(version, mediaType string, schema map[string]any) bool {
	mediaType = strings.ToLower(mediaType)
	bare := strings.TrimSpace(strings.SplitN(mediaType, ";", 2)[0])
	if strings.HasPrefix(mediaType, "text/") || bare == "application/json" || strings.HasSuffix(bare, "+json") || strings.Contains(mediaType, "xml") {
		return false
	}
	format, _ := schema["format"].(string)
	encoding, _ := schema["contentEncoding"].(string)
	legacy := version == "" || version == "3.0"
	modern := version == "" || version == "3.1" || version == "3.2"
	return legacy && strings.EqualFold(format, "binary") || modern && strings.EqualFold(encoding, "binary") || mediaType == "application/octet-stream"
}
