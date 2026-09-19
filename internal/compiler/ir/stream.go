package ir

import "strings"

// StreamPlanForMediaType normalizes one request/response media representation
// into the framing contract consumed by targets. sequential marks Media Type
// Objects that declare itemSchema or positional multipart encoding; for an
// otherwise unknown media type it selects a custom sequential protocol.
func StreamPlanForMediaType(contentType string, sequential bool) StreamPlan {
	mediaType := strings.ToLower(strings.TrimSpace(contentType))
	if index := strings.IndexByte(mediaType, ';'); index >= 0 {
		mediaType = strings.TrimSpace(mediaType[:index])
	}

	switch mediaType {
	case "application/x-ndjson", "application/ndjson", "application/jsonl", "application/json-lines":
		return StreamPlan{Framing: StreamFramingLineDelimitedJSON}
	case "text/event-stream":
		return StreamPlan{Framing: StreamFramingSSE}
	}
	if mediaType == "application/json-seq" || strings.HasSuffix(mediaType, "+json-seq") {
		return StreamPlan{Framing: StreamFramingJSONSequence}
	}
	if sequential {
		if strings.HasPrefix(mediaType, "multipart/") {
			return StreamPlan{Framing: StreamFramingMultipart}
		}
		return StreamPlan{Framing: StreamFramingCustom}
	}
	return StreamPlan{Framing: StreamFramingNone}
}
