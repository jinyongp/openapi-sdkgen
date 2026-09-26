package compatibility

import (
	"strings"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/openapiwalk"
)

const (
	RuleReference30Siblings   = "COMP-REF-001"
	RuleReference31Fields     = "COMP-REF-002"
	RuleReservedHeader        = "COMP-PARAM-001"
	RuleResponseContentType   = "COMP-RESP-001"
	RuleEncodingHeaders       = "COMP-ENC-001"
	RuleEncodingFields        = "COMP-ENC-002"
	RuleRequestBody30         = "COMP-BODY-001"
	RuleVersion30Metadata     = "COMP-VERSION-001"
	RuleVersionPre32Metadata  = "COMP-VERSION-002"
	RuleSchemaBoolean30       = "COMP-SCHEMA-001"
	RuleSchemaConst30         = "COMP-SCHEMA-002"
	RuleSchemaNullableTypes30 = "COMP-SCHEMA-003"
	RuleSchemaExclusive30     = "COMP-SCHEMA-004"
	RuleSchemaKeywords30      = "COMP-SCHEMA-005"
)

// ConsumerPolicy applies target-neutral OpenAPI consumer semantics that can be
// proven lossless before IR construction.
type ConsumerPolicy struct{}

// ConsumerPolicyMayApply cheaply identifies occurrences that can be changed or
// diagnosed by ConsumerPolicy. It lets the compiler avoid constructing JSON
// pointers and compatibility contexts for the overwhelmingly common no-op
// nodes while keeping the policy itself authoritative for the final decision.
func ConsumerPolicyMayApply(version openapidoc.VersionLine, object openapiwalk.ObjectContext, value any) bool {
	if object == openapiwalk.ObjectSchema {
		if version != openapidoc.Version30 {
			return false
		}
		if _, ok := value.(bool); ok {
			return true
		}
		schema, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := schema["type"].([]any); ok {
			return true
		}
		for _, key := range []string{"exclusiveMinimum", "exclusiveMaximum"} {
			if raw, exists := schema[key]; exists {
				if _, valid30 := raw.(bool); !valid30 {
					return true
				}
			}
		}
		for key := range schema {
			if openAPI31SchemaKeyword(key) {
				return true
			}
		}
		return false
	}
	objectValue, ok := value.(map[string]any)
	if !ok {
		return false
	}
	reference, _ := objectValue["$ref"].(string)
	hasReference := reference != ""
	switch object {
	case openapiwalk.ObjectParameter:
		return hasReference || isReservedHeaderParameter(objectValue)
	case openapiwalk.ObjectHeader, openapiwalk.ObjectEncoding:
		// Header name/ownership and Encoding media context are structural and
		// therefore require the source path even when the object has no $ref.
		return true
	case openapiwalk.ObjectRequestBody:
		return hasReference || version == openapidoc.Version30
	case openapiwalk.ObjectResponse:
		_, hasSummary := objectValue["summary"]
		return hasReference || (version != openapidoc.Version32 && hasSummary)
	case openapiwalk.ObjectExample:
		_, hasData := objectValue["dataValue"]
		_, hasSerialized := objectValue["serializedValue"]
		return hasReference || (version != openapidoc.Version32 && (hasData || hasSerialized))
	case openapiwalk.ObjectSecurityScheme:
		_, hasDeprecated := objectValue["deprecated"]
		return hasReference || (version != openapidoc.Version32 && hasDeprecated)
	case openapiwalk.ObjectLink, openapiwalk.ObjectCallback, openapiwalk.ObjectMediaType:
		return hasReference
	case openapiwalk.ObjectInfo:
		_, hasSummary := objectValue["summary"]
		return version == openapidoc.Version30 && hasSummary
	case openapiwalk.ObjectUnknown:
		if version == openapidoc.Version30 {
			if _, exists := objectValue["identifier"]; exists {
				return true
			}
		}
		if version != openapidoc.Version32 {
			if name, ok := objectValue["name"].(string); ok && name != "" {
				return true
			}
			for _, key := range []string{"summary", "parent", "kind"} {
				if _, exists := objectValue[key]; exists {
					return true
				}
			}
		}
	}
	return false
}

func (ConsumerPolicy) Apply(context Context, value any) Result {
	if context.Object == openapiwalk.ObjectSchema && context.Version == openapidoc.Version30 {
		if result, applied := applyOpenAPI30SchemaRule(context, value); applied {
			return result
		}
	}
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
	case openapiwalk.ObjectRequestBody:
		if result, applied := applyRequestBodyRule(context, object); applied {
			return result
		}
	}
	if result, applied := applyReferenceObjectRule(context, object); applied {
		return result
	}
	if result, applied := applyVersionedMetadataRule(context, object); applied {
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

func applyOpenAPI30SchemaRule(context Context, value any) (Result, bool) {
	if allowed, ok := value.(bool); ok {
		if allowed {
			return schemaCompatibilityResult(context, map[string]any{}, RuleSchemaBoolean30, ActionNormalize, "OpenAPI 3.0 boolean true schema is normalized to an empty Schema Object.", false), true
		}
		return schemaCompatibilityResult(context, map[string]any{"not": map[string]any{}}, RuleSchemaBoolean30, ActionNormalize, "OpenAPI 3.0 boolean false schema is normalized to an equivalent negated empty Schema Object.", false), true
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Result{}, false
	}

	current := object
	changed := false
	var findings []Finding
	var ledger []LedgerEntry
	copyObject := func() {
		if changed {
			return
		}
		current = make(map[string]any, len(object))
		for key, item := range object {
			current[key] = item
		}
		changed = true
	}
	record := func(rule string, action Action, pointer, message string) {
		findings = append(findings, Finding{
			RuleID:      rule,
			Conformance: ConformanceNonconforming,
			Disposition: DispositionNotDefined,
			Action:      action,
			Impact:      ImpactValidation,
			Source:      context.Source,
			Pointer:     pointer,
			Message:     message,
		})
		ledger = append(ledger, LedgerEntry{
			RuleID:  rule,
			Action:  action,
			Impact:  ImpactValidation,
			Source:  context.Source,
			Pointer: pointer,
			Version: context.Version,
		})
	}

	if types, isArray := object["type"].([]any); isArray {
		nonNull, valid := nullableOpenAPI30Type(types)
		if !valid || object["nullable"] == true {
			return schemaCompatibilityResult(context, value, RuleSchemaNullableTypes30, ActionReject, "OpenAPI 3.0 type array is outside the proved nullable two-type normalization.", true), true
		}
		copyObject()
		current["type"] = nonNull
		current["nullable"] = true
		record(RuleSchemaNullableTypes30, ActionNormalize, context.Pointer+"/type", "Nullable two-type array is normalized to OpenAPI 3.0 nullable semantics.")
	}

	if constant, exists := object["const"]; exists {
		if _, hasEnum := object["enum"]; hasEnum {
			return schemaCompatibilityResult(context, value, RuleSchemaConst30, ActionReject, "OpenAPI 3.0 const combined with enum is outside the proved normalization.", true), true
		}
		copyObject()
		delete(current, "const")
		current["enum"] = []any{constant}
		record(RuleSchemaConst30, ActionNormalize, context.Pointer+"/const", "const is normalized to an equivalent single-value enum for OpenAPI 3.0.")
	}

	for _, pair := range []struct {
		exclusive string
		bound     string
	}{
		{exclusive: "exclusiveMinimum", bound: "minimum"},
		{exclusive: "exclusiveMaximum", bound: "maximum"},
	} {
		raw, exists := object[pair.exclusive]
		if !exists {
			continue
		}
		if _, isBoolean := raw.(bool); isBoolean {
			continue
		}
		if !isJSONNumber(raw) {
			return schemaCompatibilityResult(context, value, RuleSchemaExclusive30, ActionReject, "OpenAPI 3.0 numeric exclusive bound is not representable by the proved lowering.", true), true
		}
		if _, hasBound := object[pair.bound]; hasBound {
			return schemaCompatibilityResult(context, value, RuleSchemaExclusive30, ActionReject, "OpenAPI 3.0 numeric exclusive bound combined with an existing bound requires explicit bound algebra.", true), true
		}
		copyObject()
		current[pair.bound] = raw
		current[pair.exclusive] = true
		record(RuleSchemaExclusive30, ActionNormalize, context.Pointer+"/"+pair.exclusive, "Numeric exclusive bound is normalized to the equivalent OpenAPI 3.0 bound plus boolean exclusive flag.")
	}

	for _, key := range []string{"$comment", "examples"} {
		if _, exists := object[key]; !exists {
			continue
		}
		copyObject()
		delete(current, key)
		record(RuleSchemaKeywords30, ActionIgnore, context.Pointer+"/"+key, key+" is not defined by OpenAPI 3.0 and is ignored for generated semantics.")
	}

	for key := range object {
		if !openAPI31SchemaKeyword(key) || key == "const" || key == "$comment" || key == "examples" {
			continue
		}
		return schemaCompatibilityResult(context, value, RuleSchemaKeywords30, ActionReject, "JSON Schema keyword "+key+" has no proved OpenAPI 3.0 equivalent.", true), true
	}

	if !changed {
		return Result{}, false
	}
	return Result{Value: current, Findings: findings, Ledger: ledger, Changed: true}, true
}

func nullableOpenAPI30Type(values []any) (string, bool) {
	if len(values) != 2 {
		return "", false
	}
	var nonNull string
	nulls := 0
	for _, raw := range values {
		value, ok := raw.(string)
		if !ok {
			return "", false
		}
		if value == "null" {
			nulls++
			continue
		}
		switch value {
		case "array", "boolean", "integer", "number", "object", "string":
		default:
			return "", false
		}
		if nonNull != "" {
			return "", false
		}
		nonNull = value
	}
	return nonNull, nulls == 1 && nonNull != ""
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func openAPI31SchemaKeyword(key string) bool {
	switch key {
	case "$anchor", "$comment", "$defs", "$dynamicAnchor", "$dynamicRef", "$id", "$schema", "$vocabulary",
		"const", "contains", "contentEncoding", "contentMediaType", "contentSchema", "dependentRequired", "dependentSchemas",
		"else", "examples", "if", "maxContains", "minContains", "patternProperties", "prefixItems", "propertyNames", "then",
		"unevaluatedItems", "unevaluatedProperties":
		return true
	default:
		return false
	}
}

func schemaCompatibilityResult(context Context, value any, rule string, action Action, message string, reject bool) Result {
	scope := failure.ScopeNone
	effect := failure.EffectNone
	if reject {
		scope = failure.ScopeDocument
		effect = failure.EffectBlock
	}
	return Result{
		Value:   value,
		Reject:  reject,
		Scope:   scope,
		Effect:  effect,
		Changed: action == ActionNormalize || action == ActionIgnore,
		Findings: []Finding{{
			RuleID:      rule,
			Conformance: ConformanceNonconforming,
			Disposition: DispositionNotDefined,
			Action:      action,
			Impact:      ImpactValidation,
			Scope:       scope,
			Effect:      effect,
			Source:      context.Source,
			Pointer:     context.Pointer,
			Message:     message,
		}},
		Ledger: []LedgerEntry{{
			RuleID:  rule,
			Action:  action,
			Impact:  ImpactValidation,
			Scope:   scope,
			Effect:  effect,
			Source:  context.Source,
			Pointer: context.Pointer,
			Version: context.Version,
		}},
	}
}

func applyVersionedMetadataRule(context Context, object map[string]any) (Result, bool) {
	if context.Version == "" {
		return Result{}, false
	}
	if context.Version == openapidoc.Version30 {
		switch context.Pointer {
		case "#/info":
			if result, applied := ignoreNonconformingFields(context, object, RuleVersion30Metadata, ImpactAnnotation, "summary"); applied {
				return result, true
			}
		case "#/info/license":
			if result, applied := ignoreNonconformingFields(context, object, RuleVersion30Metadata, ImpactMetadataOnly, "identifier"); applied {
				return result, true
			}
		}
	}
	if context.Version == openapidoc.Version32 {
		return Result{}, false
	}
	if context.Object == openapiwalk.ObjectSecurityScheme {
		if result, applied := ignoreNonconformingFields(context, object, RuleVersionPre32Metadata, ImpactAnnotation, "deprecated"); applied {
			return result, true
		}
	}
	if context.Object == openapiwalk.ObjectExample {
		if result, applied := ignoreNonconformingFields(context, object, RuleVersionPre32Metadata, ImpactMetadataOnly, "dataValue", "serializedValue"); applied {
			return result, true
		}
	}
	if context.Object == openapiwalk.ObjectResponse {
		if result, applied := ignoreNonconformingFields(context, object, RuleVersionPre32Metadata, ImpactAnnotation, "summary"); applied {
			return result, true
		}
	}
	tokens, ok := pointerTokens(context.Pointer)
	if !ok || len(tokens) == 0 {
		return Result{}, false
	}
	if tokens[len(tokens)-1] == "server" || (len(tokens) >= 2 && tokens[len(tokens)-2] == "servers") {
		if result, applied := ignoreNonconformingFields(context, object, RuleVersionPre32Metadata, ImpactAnnotation, "name"); applied {
			return result, true
		}
	}
	if len(tokens) >= 2 && tokens[len(tokens)-2] == "tags" {
		if result, applied := ignoreNonconformingFields(context, object, RuleVersionPre32Metadata, ImpactAnnotation, "summary", "parent", "kind"); applied {
			return result, true
		}
	}
	return Result{}, false
}

func ignoreNonconformingFields(context Context, object map[string]any, rule string, impact SemanticImpact, fields ...string) (Result, bool) {
	var filtered map[string]any
	var findings []Finding
	var ledger []LedgerEntry
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			continue
		}
		if filtered == nil {
			filtered = make(map[string]any, len(object))
			for key, value := range object {
				filtered[key] = value
			}
		}
		delete(filtered, field)
		pointer := context.Pointer + "/" + field
		findings = append(findings, Finding{
			RuleID:      rule,
			Conformance: ConformanceNonconforming,
			Disposition: DispositionNotDefined,
			Action:      ActionIgnore,
			Impact:      impact,
			Source:      context.Source,
			Pointer:     pointer,
			Message:     field + " is not defined by the declared OpenAPI version and is ignored for generated semantics.",
		})
		ledger = append(ledger, LedgerEntry{
			RuleID:  rule,
			Action:  ActionIgnore,
			Impact:  impact,
			Source:  context.Source,
			Pointer: pointer,
			Version: context.Version,
		})
	}
	if filtered == nil {
		return Result{}, false
	}
	return Result{Value: filtered, Findings: findings, Ledger: ledger, Changed: true}, true
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

func applyRequestBodyRule(context Context, object map[string]any) (Result, bool) {
	if context.Version != openapidoc.Version30 {
		return Result{}, false
	}
	method, ok := requestBodyMethod(context.Pointer)
	if !ok {
		return Result{}, false
	}
	switch method {
	case "get", "head", "delete", "options", "trace":
	default:
		return Result{}, false
	}
	if !requestBodyPotentiallyMeaningful(object) {
		return omit(context, object, RuleRequestBody30, ImpactWire), true
	}
	if method == "delete" {
		return compatibilityFindingResult(
			context,
			object,
			RuleRequestBody30,
			ActionPreserveExtension,
			DispositionIgnored,
			"OpenAPI 3.0 DELETE request body is preserved as an evidenced compatibility extension.",
			false,
		), true
	}
	return compatibilityFindingResultWithFailure(
		context,
		object,
		RuleRequestBody30,
		ActionReject,
		DispositionIgnored,
		"OpenAPI 3.0 request body is potentially meaningful on a method whose payload semantics are not portable.",
		failure.ScopeOperation,
		failure.EffectBlock,
		true,
	), true
}

func requestBodyMethod(pointer string) (string, bool) {
	tokens, ok := pointerTokens(pointer)
	if !ok || len(tokens) < 2 || tokens[len(tokens)-1] != "requestBody" {
		return "", false
	}
	return strings.ToLower(tokens[len(tokens)-2]), true
}

func requestBodyPotentiallyMeaningful(object map[string]any) bool {
	if reference, _ := object["$ref"].(string); reference != "" {
		return true
	}
	if required, _ := object["required"].(bool); required {
		return true
	}
	content, _ := object["content"].(map[string]any)
	for _, rawMedia := range content {
		media, ok := rawMedia.(map[string]any)
		if !ok {
			return true
		}
		if encoding, ok := media["encoding"].(map[string]any); ok && len(encoding) != 0 {
			return true
		}
		if schema, exists := media["schema"]; exists && !requestBodySchemaStructurallyEmpty(schema) {
			return true
		}
	}
	return false
}

func requestBodySchemaStructurallyEmpty(value any) bool {
	schema, ok := value.(map[string]any)
	if !ok {
		return value == nil
	}
	for key, raw := range schema {
		switch key {
		case "title", "description", "example", "examples", "deprecated", "readOnly", "writeOnly":
			continue
		case "type":
			if raw == "object" {
				continue
			}
			return false
		case "properties":
			properties, ok := raw.(map[string]any)
			if ok && len(properties) == 0 {
				continue
			}
			return false
		case "required":
			required, ok := raw.([]any)
			if ok && len(required) == 0 {
				continue
			}
			return false
		case "additionalProperties":
			if allowed, ok := raw.(bool); ok && !allowed {
				continue
			}
			return false
		case "nullable":
			if nullable, ok := raw.(bool); ok && !nullable {
				continue
			}
			return false
		default:
			if strings.HasPrefix(key, "x-") {
				continue
			}
			return false
		}
	}
	return true
}

func compatibilityFindingResult(context Context, value any, rule string, action Action, disposition NormativeDisposition, message string, reject bool) Result {
	scope := failure.ScopeNone
	effect := failure.EffectNone
	if reject {
		scope = failure.ScopeDocument
		effect = failure.EffectBlock
	}
	return compatibilityFindingResultWithFailure(context, value, rule, action, disposition, message, scope, effect, reject)
}

func compatibilityFindingResultWithFailure(
	context Context,
	value any,
	rule string,
	action Action,
	disposition NormativeDisposition,
	message string,
	scope failure.Scope,
	effect failure.Effect,
	reject bool,
) Result {
	return Result{
		Value:  value,
		Reject: reject,
		Scope:  scope,
		Effect: effect,
		Findings: []Finding{{
			RuleID:      rule,
			Conformance: ConformanceConforming,
			Disposition: disposition,
			Action:      action,
			Impact:      ImpactWire,
			Scope:       scope,
			Effect:      effect,
			Source:      context.Source,
			Pointer:     context.Pointer,
			Message:     message,
		}},
		Ledger: []LedgerEntry{{
			RuleID:  rule,
			Action:  action,
			Impact:  ImpactWire,
			Scope:   scope,
			Effect:  effect,
			Source:  context.Source,
			Pointer: context.Pointer,
			Version: context.Version,
		}},
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
	if context.Version != openapidoc.Version30 {
		_, hasStyle := object["style"]
		_, hasExplode := object["explode"]
		_, hasAllowReserved := object["allowReserved"]
		if hasStyle || hasExplode || hasAllowReserved {
			remove["contentType"] = true
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
