package ir

import "strings"

// StreamPlanForMediaType normalizes one request/response media representation
// into the framing contract consumed by targets. itemSchema presence marks an
// otherwise unknown media type as a custom sequential protocol.
func StreamPlanForMediaType(contentType string, hasItemSchema bool) StreamPlan {
	mediaType := strings.ToLower(strings.TrimSpace(contentType))
	if index := strings.IndexByte(mediaType, ';'); index >= 0 {
		mediaType = strings.TrimSpace(mediaType[:index])
	}

	switch mediaType {
	case "application/x-ndjson", "application/ndjson", "application/jsonl", "application/json-lines":
		return StreamPlan{Framing: StreamFramingLineDelimitedJSON}
	case "application/json-seq":
		return StreamPlan{Framing: StreamFramingJSONSequence}
	case "text/event-stream":
		return StreamPlan{Framing: StreamFramingSSE}
	}
	if hasItemSchema {
		if strings.HasPrefix(mediaType, "multipart/") {
			return StreamPlan{Framing: StreamFramingMultipart}
		}
		return StreamPlan{Framing: StreamFramingCustom}
	}
	return StreamPlan{Framing: StreamFramingNone}
}
