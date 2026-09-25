package compatibility

import (
	"strings"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

const (
	RuleReference30Siblings = "COMP-REF-001"
	RuleReference31Fields   = "COMP-REF-002"
	RuleReservedHeader      = "COMP-PARAM-001"
	RuleResponseContentType = "COMP-RESP-001"
	RuleEncodingHeaders     = "COMP-ENC-001"
	RuleEncodingFields      = "COMP-ENC-002"
)

// ConsumerPolicy applies target-neutral OpenAPI consumer semantics that can be
// proven lossless before IR construction.
type ConsumerPolicy struct{}

func (ConsumerPolicy) Apply(context Context, value any) Result {
	object, ok := value.(map[string]any)
	if !ok {
		return Result{Value: value}
	}
	switch context.Object {
	case openapiwalk.ObjectParameter:
		if isReservedHeaderParameter(object) {
			return omit(context, value, RuleReservedHeader, ImpactWire)
		}
	case openapiwalk.ObjectHeader:
		if name, owner, ok := headerOccurrence(context.Pointer); ok && strings.EqualFold(name, "Content-Type") {
			switch owner {
			case "response":
				return omit(context, value, RuleResponseContentType, ImpactWire)
			case "encoding":
				return omit(context, value, RuleEncodingHeaders, ImpactWire)
			}
		}
	case openapiwalk.ObjectEncoding:
		return applyEncodingRule(context, object)
	}
	if result, applied := applyReferenceObjectRule(context, object); applied {
		return result
	}
	return Result{Value: value}
}

func applyReferenceObjectRule(context Context, object map[string]any) (Result, bool) {
	reference, _ := object["$ref"].(string)
	if reference == "" || !isReferenceObjectContext(context.Object) {
		return Result{}, false
	}
	allowed := map[string]bool{"$ref": true}
	rule := RuleReference30Siblings
	if context.Version == openapidoc.Version31 || context.Version == openapidoc.Version32 {
		allowed["summary"] = true
		allowed["description"] = true
		rule = RuleReference31Fields
	}
	filtered, changed := keepObjectFields(object, allowed)
	if !changed {
		return Result{Value: object}, true
	}
	return changedResult(context, filtered, rule, ImpactReference), true
}

func isReferenceObjectContext(object openapiwalk.ObjectContext) bool {
	switch object {
	case openapiwalk.ObjectParameter,
		openapiwalk.ObjectHeader,
		openapiwalk.ObjectRequestBody,
		openapiwalk.ObjectResponse,
		openapiwalk.ObjectLink,
		openapiwalk.ObjectCallback,
		openapiwalk.ObjectExample,
		openapiwalk.ObjectSecurityScheme,
		openapiwalk.ObjectMediaType:
		return true
	default:
		return false
	}
}

func isReservedHeaderParameter(object map[string]any) bool {
	if reference, _ := object["$ref"].(string); reference != "" {
		return false
	}
	location, _ := object["in"].(string)
	name, _ := object["name"].(string)
	if !strings.EqualFold(location, "header") {
		return false
	}
	switch {
	case strings.EqualFold(name, "Accept"):
		return true
	case strings.EqualFold(name, "Content-Type"):
		return true
	case strings.EqualFold(name, "Authorization"):
		return true
	default:
		return false
	}
}

func applyEncodingRule(context Context, object map[string]any) Result {
	mediaType, ok := encodingMediaType(context.Pointer)
	if !ok {
		return Result{Value: object}
	}
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	multipart := strings.HasPrefix(normalized, "multipart/")
	form := normalized == "application/x-www-form-urlencoded"
	if !multipart && !form {
		return omit(context, object, RuleEncodingFields, ImpactWire)
	}

	remove := map[string]bool{}
	if form {
		remove["headers"] = true
	} else {
		remove["allowReserved"] = true
		if context.Version == openapidoc.Version30 {
			remove["style"] = true
			remove["explode"] = true
		}
	}
	filtered, changed := removeObjectFields(object, remove)
	if !changed {
		return Result{Value: object}
	}
	rule := RuleEncodingFields
	if remove["headers"] {
		rule = RuleEncodingHeaders
	}
	return changedResult(context, filtered, rule, ImpactWire)
}

func omit(context Context, value any, rule string, impact SemanticImpact) Result {
	return Result{
		Value:   value,
		Omit:    true,
		Changed: true,
		Ledger: []LedgerEntry{{
			RuleID:  rule,
			Action:  ActionIgnore,
			Impact:  impact,
			Source:  context.Source,
			Pointer: context.Pointer,
			Version: context.Version,
		}},
	}
}

func changedResult(context Context, value any, rule string, impact SemanticImpact) Result {
	return Result{
		Value:   value,
		Changed: true,
		Ledger: []LedgerEntry{{
			RuleID:  rule,
			Action:  ActionIgnore,
			Impact:  impact,
			Source:  context.Source,
			Pointer: context.Pointer,
			Version: context.Version,
		}},
	}
}

func keepObjectFields(object map[string]any, allowed map[string]bool) (map[string]any, bool) {
	for key := range object {
		if !allowed[key] {
			result := make(map[string]any, len(allowed))
			for name, value := range object {
				if allowed[name] {
					result[name] = value
				}
			}
			return result, true
		}
	}
	return object, false
}

func removeObjectFields(object map[string]any, remove map[string]bool) (map[string]any, bool) {
	changed := false
	for key := range remove {
		if _, exists := object[key]; exists {
			changed = true
			break
		}
	}
	if !changed {
		return object, false
	}
	result := make(map[string]any, len(object))
	for key, value := range object {
		if !remove[key] {
			result[key] = value
		}
	}
	return result, true
}

func headerOccurrence(pointer string) (name, owner string, ok bool) {
	tokens, ok := pointerTokens(pointer)
	if !ok || len(tokens) < 2 || tokens[len(tokens)-2] != "headers" {
		return "", "", false
	}
	name = tokens[len(tokens)-1]
	for index := len(tokens) - 3; index >= 0; index-- {
		switch tokens[index] {
		case "encoding", "prefixEncoding", "itemEncoding":
			return name, "encoding", true
		case "responses":
			return name, "response", true
		case "components":
			return "", "", false
		}
	}
	return "", "", false
}

func encodingMediaType(pointer string) (string, bool) {
	tokens, ok := pointerTokens(pointer)
	if !ok {
		return "", false
	}
	for index := len(tokens) - 1; index >= 2; index-- {
		if tokens[index-1] == "encoding" && tokens[index-2] != "" {
			for mediaIndex := index - 2; mediaIndex >= 1; mediaIndex-- {
				if tokens[mediaIndex-1] == "content" {
					return tokens[mediaIndex], true
				}
			}
			return "", false
		}
	}
	return "", false
}

func pointerTokens(pointer string) ([]string, bool) {
	if pointer == "#" {
		return nil, true
	}
	if !strings.HasPrefix(pointer, "#/") {
		return nil, false
	}
	encoded := strings.Split(strings.TrimPrefix(pointer, "#/"), "/")
	result := make([]string, 0, len(encoded))
	for _, token := range encoded {
		decoded, ok := decodePointerToken(token)
		if !ok {
			return nil, false
		}
		result = append(result, decoded)
	}
	return result, true
}

func decodePointerToken(value string) (string, bool) {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '~' {
			result.WriteByte(value[index])
			continue
		}
		if index+1 >= len(value) {
			return "", false
		}
		index++
		switch value[index] {
		case '0':
			result.WriteByte('~')
		case '1':
			result.WriteByte('/')
		default:
			return "", false
		}
	}
	return result.String(), true
}
