package typescript

import (
	"sort"
	"strings"
)

type runtimeHandlerImport struct {
	path  string
	names []string
}

type runtimeWireHandlers struct {
	imports []runtimeHandlerImport
	fields  []string
}

type runtimeSecurityHandlers struct {
	imports []runtimeHandlerImport
	fields  []string
}

func prepareWireHandlers(features []runtimeFeature) runtimeWireHandlers {
	set := make(runtimeFeatureSet)
	for _, feature := range features {
		set[feature] = true
	}
	imports := make(map[string]map[string]bool)
	var fields []string
	add := func(source string, names []string, field string) {
		if imports[source] == nil {
			imports[source] = make(map[string]bool)
		}
		for _, name := range names {
			imports[source][name] = true
		}
		fields = append(fields, field)
	}
	has := func(names ...string) bool {
		for _, name := range names {
			if set[runtimeFeature("schema."+name)] {
				return true
			}
		}
		return false
	}
	if has("dynamic") {
		add("wire-dynamic", []string{"extendDynamicScope", "resolveDynamicReference"}, "dynamic: { extend: extendDynamicScope, resolve: resolveDynamicReference }")
	}
	if has("contentSchema") {
		add("wire-content", []string{"decodeSchemaContent"}, "decodeContent: decodeSchemaContent")
	}
	if has("const", "enum") {
		add("wire-literal", []string{"validateLiteral"}, "literal: validateLiteral")
	}
	if has("maximum", "minimum", "exclusiveMaximum", "exclusiveMinimum") {
		add("wire-number", []string{"validateNumber"}, "number: validateNumber")
	}
	if has("minLength", "maxLength") {
		add("wire-string", []string{"validateString"}, "string: validateString")
	}
	if has("multipleOf") {
		add("wire-multiple-of", []string{"validateMultipleOf"}, "multipleOf: validateMultipleOf")
	}
	if has("pattern") {
		add("wire-string-pattern", []string{"validateStringPattern"}, "stringPattern: validateStringPattern")
	}
	if has("allOf", "oneOf", "anyOf", "not", "if", "then", "else", "discriminator") {
		add("wire-composition", []string{"validateComposition", "transformComposition"}, "composition: validateComposition, transformComposition: transformComposition")
	}
	for _, group := range []struct {
		features      []string
		source, field string
	}{
		{[]string{"minItems", "maxItems"}, "wire-array-limits", "arrayBefore"},
		{[]string{"uniqueItems"}, "wire-array-unique", "arrayUnique"},
		{[]string{"contains", "minContains", "maxContains"}, "wire-array-contains", "arrayContains"},
		{[]string{"unevaluatedItems"}, "wire-array-unevaluated", "arrayAfter"},
	} {
		if has(group.features...) {
			add(group.source, []string{group.field}, group.field+": "+group.field)
		}
	}
	for _, group := range []struct {
		features []string
		field    string
	}{
		{[]string{"minProperties", "maxProperties"}, "objectBefore"},
		{[]string{"dependentRequired", "dependentSchemas"}, "dependencies"},
		{[]string{"propertyNames"}, "propertyNames"},
		{[]string{"unevaluatedProperties"}, "objectAfter"},
	} {
		if has(group.features...) {
			sources := map[string]string{"objectBefore": "wire-object-limits", "dependencies": "wire-object-dependencies", "propertyNames": "wire-property-names", "objectAfter": "wire-object-unevaluated"}
			add(sources[group.field], []string{group.field}, group.field+": "+group.field)
		}
	}
	if has("patternProperties") {
		add("wire-pattern", []string{"matchingPropertySchemas"}, "patterns: matchingPropertySchemas")
	}
	var cases []string
	for _, feature := range features {
		if !strings.HasPrefix(string(feature), "schema.format.") {
			continue
		}
		name := strings.ToLower(strings.TrimPrefix(string(feature), "schema.format."))
		format, ok := wireFormatHandler(name)
		if !ok {
			continue
		}
		if imports[format.path] == nil {
			imports[format.path] = make(map[string]bool)
		}
		imports[format.path][format.name] = true
		cases = append(cases, "case "+quoteTS(name)+": return "+format.name+"(value"+format.arguments+");")
	}
	if len(cases) > 0 {
		sort.Strings(cases)
		fields = append(fields, "format: (value: string, format: string): boolean => { switch (format.toLowerCase()) { "+strings.Join(uniqueStrings(cases), " ")+" default: return true } }")
	}
	return runtimeWireHandlers{imports: freezeRuntimeHandlerImports(imports), fields: fields}
}

type runtimeFormatHandler struct{ path, name, arguments string }

func wireFormatHandler(name string) (runtimeFormatHandler, bool) {
	known := map[string]string{"date": "matchesWireDate", "time": "matchesWireTime", "date-time": "matchesWireDateTime", "duration": "matchesWireDuration", "email": "matchesWireEmail", "idn-email": "matchesWireIDNEmail", "hostname": "matchesWireHostname", "idn-hostname": "matchesWireIDNHostname", "ipv4": "matchesWireIPv4", "ipv6": "matchesWireIPv6", "uuid": "matchesWireUUID", "uri-template": "matchesWireURITemplate", "json-pointer": "matchesWireJSONPointer", "relative-json-pointer": "matchesWireRelativeJSONPointer", "regex": "matchesWireRegex"}
	if function, ok := known[name]; ok {
		return runtimeFormatHandler{path: "wire-format-" + name, name: function}, true
	}
	for _, value := range []struct{ name, args string }{{"uri", ", true, false"}, {"uri-reference", ", false, false"}, {"iri", ", true, true"}, {"iri-reference", ", false, true"}} {
		if name == value.name {
			return runtimeFormatHandler{path: "wire-format-uri", name: "matchesWireURI", arguments: value.args}, true
		}
	}
	return runtimeFormatHandler{}, false
}

func prepareSecurityHandlers(features []runtimeFeature) runtimeSecurityHandlers {
	imports := make(map[string]map[string]bool)
	fields := make(map[string]string)
	for _, feature := range features {
		if !strings.HasPrefix(string(feature), "security.") {
			continue
		}
		kind := strings.TrimPrefix(string(feature), "security.")
		if strings.HasPrefix(kind, "http.") && kind != "http.basic" && kind != "http.bearer" {
			kind = "http"
		}
		var source, name string
		switch kind {
		case "apiKey.header":
			source, name = "security-api-header", "apiHeader"
		case "apiKey.query":
			source, name = "security-api-query", "apiQuery"
		case "apiKey.cookie":
			source, name = "security-api-cookie", "apiCookie"
		case "http.basic":
			source, name = "security-basic", "httpBasic"
		case "http.bearer":
			source, name = "security-bearer", "httpBearer"
		case "http":
			source, name = "security-http", "httpCredential"
		case "oauth2", "openIdConnect":
			source, name = "security-oauth", "oauthCredential"
		case "mutualTLS":
			source, name = "security-mtls", "mutualTLS"
		default:
			continue
		}
		if imports[source] == nil {
			imports[source] = make(map[string]bool)
		}
		imports[source][name] = true
		fields[kind] = quoteTS(kind) + ": " + name
	}
	result := runtimeSecurityHandlers{imports: freezeRuntimeHandlerImports(imports)}
	for _, key := range sortedRuntimeHandlerKeys(fields) {
		result.fields = append(result.fields, fields[key])
	}
	return result
}

func freezeRuntimeHandlerImports(imports map[string]map[string]bool) []runtimeHandlerImport {
	var result []runtimeHandlerImport
	for _, source := range sortedRuntimeHandlerKeys(imports) {
		artifact := "internal/runtime/" + source + ".ts"
		if !strings.Contains(source, "/") {
			artifact = runtimeTemplatePath(source + ".ts")
		}
		result = append(result, runtimeHandlerImport{path: artifact, names: sortedStringKeys(imports[source])})
	}
	return result
}

func sortedRuntimeHandlerKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
