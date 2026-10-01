package sdkgen

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/openapiwalk"
)

const (
	dynamicAnchorMetadataKey    = "x-sdkgen-dynamic-anchor"
	dynamicReferenceMetadataKey = "x-sdkgen-dynamic-reference"
)

// normalizeNestedSchemaReferences lowers references to Schema locations into
// named identities before targets run. Original component names remain stable;
// other locations share one synthetic component, including recursive targets.
func normalizeNestedSchemaReferences(document any) (any, error) {
	root, ok := document.(map[string]any)
	if !ok {
		return document, nil
	}
	anchored, err := normalizeSchemaAnchorReferences(root)
	if err != nil {
		return nil, err
	}
	root, ok = anchored.(map[string]any)
	if !ok {
		return anchored, nil
	}
	return normalizeSchemaLocationReferences(root)
}

// normalizeSchemaAnchorReferences resolves local JSON Schema anchors before
// target lowering. TypeScript's wire descriptor model addresses schemas by
// JSON Pointer, so keeping an anchor or a dynamic reference until target
// emission would silently discard its reference semantics. A dynamic
// reference whose dynamic scope is wholly represented in this document is
// equivalent to the canonical pointer selected here; references that cannot
// be bound locally remain an explicit compilation error.
func normalizeSchemaAnchorReferences(document map[string]any) (any, error) {
	base := schemaDocumentResourceURI(document)
	index := schemaAnchorIndex{anchors: map[string]string{}, resources: map[string]string{base: ""}}
	if err := index.collect(document, "", base); err != nil {
		return nil, err
	}
	return index.rewrite(document, "", base)
}

type schemaAnchorIndex struct {
	anchors   map[string]string
	resources map[string]string
}

func (index schemaAnchorIndex) collect(value any, pointer, resource string) error {
	switch typed := value.(type) {
	case map[string]any:
		tokens, tokenErr := schemaPointerTokens(pointer)
		current := resource
		if tokenErr == nil && openapiwalk.ObjectContextAt(tokens) == openapiwalk.ObjectSchema {
			if identifier, _ := typed["$id"].(string); identifier != "" {
				current = resolveSchemaResourceURI(resource, identifier)
				if existing, exists := index.resources[current]; exists && existing != pointer {
					return fmt.Errorf("duplicate JSON Schema resource %q", current)
				}
				index.resources[current] = pointer
			}
			for _, keyword := range []string{"$anchor", "$dynamicAnchor"} {
				anchor, _ := typed[keyword].(string)
				if anchor == "" {
					continue
				}
				name := schemaAnchorURI(current, anchor)
				if existing, exists := index.anchors[name]; exists && existing != schemaLocalReference(pointer) {
					return fmt.Errorf("duplicate JSON Schema anchor %q at %s and %s", anchor, existing, "#"+pointer)
				}
				index.anchors[name] = schemaLocalReference(pointer)
			}
		}
		for key, child := range typed {
			if tokenErr != nil || openapiwalk.ReferenceChildOpaque(tokens, key, child) {
				continue
			}
			if err := index.collect(child, appendSchemaPointer(pointer, key), current); err != nil {
				return err
			}
		}
	case []any:
		for offset, child := range typed {
			if err := index.collect(child, appendSchemaPointer(pointer, fmt.Sprint(offset)), resource); err != nil {
				return err
			}
		}
	}
	return nil
}

func (index schemaAnchorIndex) rewrite(value any, pointer, resource string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		tokens, tokenErr := schemaPointerTokens(pointer)
		current := resource
		candidate := tokenErr == nil && openapiwalk.ObjectContextAt(tokens) == openapiwalk.ObjectSchema
		if candidate {
			if identifier, _ := typed["$id"].(string); identifier != "" {
				current = resolveSchemaResourceURI(resource, identifier)
			}
		}
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if candidate && key == "$anchor" {
				continue
			}
			if candidate && key == "$dynamicRef" {
				reference, _ := child.(string)
				resolved, found := index.resolve(current, reference)
				if !found {
					return nil, fmt.Errorf("unresolved local JSON Schema dynamic reference %q at #%s", reference, pointer)
				}
				anchor, _ := schemaReferenceAnchor(reference)
				result[dynamicReferenceMetadataKey] = map[string]any{"anchor": anchor, "reference": resolved}
				continue
			}
			if candidate && key == "$dynamicAnchor" {
				if anchor, _ := child.(string); anchor != "" {
					result[dynamicAnchorMetadataKey] = anchor
				}
				continue
			}
			if candidate && key == "$ref" {
				reference, _ := child.(string)
				resolved, found := index.resolve(current, reference)
				if found {
					result[key] = resolved
					continue
				}
			}
			if tokenErr != nil || openapiwalk.ReferenceChildOpaque(tokens, key, child) {
				result[key] = child
				continue
			}
			normalized, err := index.rewrite(child, appendSchemaPointer(pointer, key), current)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for offset, child := range typed {
			normalized, err := index.rewrite(child, appendSchemaPointer(pointer, fmt.Sprint(offset)), resource)
			if err != nil {
				return nil, err
			}
			result[offset] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
}

func schemaReferenceAnchor(reference string) (string, bool) {
	_, anchor, found := strings.Cut(reference, "#")
	var err error
	anchor, err = url.PathUnescape(anchor)
	if !found || err != nil || anchor == "" || strings.Contains(anchor, "/") {
		return "", false
	}
	return anchor, true
}

func (index schemaAnchorIndex) resolve(resource, reference string) (string, bool) {
	if reference == "" {
		return "", false
	}
	before, fragment, _ := strings.Cut(reference, "#")
	resource = resolveSchemaResourceURI(resource, before)
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		return "", false
	}
	if decoded == "" || strings.HasPrefix(decoded, "/") {
		// Component pointers are the compiler's existing canonical identity
		// form, including pointers lowered from contained/remote resources.
		if before == "" && strings.HasPrefix(decoded, "/components/schemas/") {
			return schemaLocalReference(decoded), true
		}
		pointer, ok := index.resources[resource]
		if ok {
			return schemaLocalReference(pointer + decoded), true
		}
		return "", false
	}
	resolved, ok := index.anchors[schemaAnchorURI(resource, decoded)]
	return resolved, ok
}

func schemaPointerContext(pointer string) openapiwalk.ObjectContext {
	tokens, err := schemaPointerTokens(pointer)
	if err != nil {
		return openapiwalk.ObjectUnknown
	}
	return openapiwalk.ObjectContextAt(tokens)
}

func schemaPointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for index, token := range tokens {
		decoded, err := decodeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		tokens[index] = decoded
	}
	return tokens, nil
}

func schemaLocalReference(pointer string) string {
	value := url.URL{Fragment: pointer}
	return "#" + value.EscapedFragment()
}

func schemaDocumentResourceURI(document map[string]any) string {
	for _, key := range []string{"$self", "$id"} {
		if value, _ := document[key].(string); value != "" {
			return strings.TrimSuffix(strings.SplitN(value, "#", 2)[0], "#")
		}
	}
	return "urn:openapi-sdkgen:document"
}

func schemaAnchorURI(resource, anchor string) string {
	return strings.SplitN(resource, "#", 2)[0] + "#" + anchor
}

func resolveSchemaResourceURI(base, identifier string) string {
	if identifier == "" {
		return strings.SplitN(base, "#", 2)[0]
	}
	if strings.HasPrefix(identifier, "#") {
		return strings.SplitN(base, "#", 2)[0] + identifier
	}
	baseURL, baseErr := url.Parse(strings.SplitN(base, "#", 2)[0])
	identifierURL, identifierErr := url.Parse(identifier)
	if baseErr != nil || identifierErr != nil || strings.HasPrefix(base, "urn:") {
		return identifier
	}
	return baseURL.ResolveReference(identifierURL).String()
}

func appendSchemaPointer(pointer, token string) string {
	encoded := strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
	return pointer + "/" + encoded
}

type schemaLocationNormalizer struct {
	root    map[string]any
	names   map[string]string
	aliases map[string]any
	paths   map[string]string
	used    map[string]bool
}

func normalizeSchemaLocationReferences(root map[string]any) (any, error) {
	state := schemaLocationNormalizer{root: root, names: map[string]string{}, aliases: map[string]any{}, paths: map[string]string{}, used: map[string]bool{}}
	components, _ := root["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for name := range schemas {
		state.used[name] = true
	}
	result, err := state.rewrite(root, "")
	if err != nil {
		return nil, err
	}
	// Each source location is assigned once. Rewriting an alias never unfolds
	// its referenced schema, so recursive graphs stay bounded and named.
	processed := map[string]bool{}
	for len(processed) < len(state.aliases) {
		for _, name := range sortedOpaqueMapKeys(state.aliases) {
			if processed[name] {
				continue
			}
			value, err := state.rewrite(state.aliases[name], state.paths[name])
			if err != nil {
				return nil, err
			}
			state.aliases[name] = value
			processed[name] = true
		}
	}
	output := result.(map[string]any)
	if len(state.aliases) == 0 {
		return output, nil
	}
	components, _ = output["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
		output["components"] = components
	}
	schemas, _ = components["schemas"].(map[string]any)
	if schemas == nil {
		schemas = map[string]any{}
		components["schemas"] = schemas
	}
	for name, value := range state.aliases {
		schemas[name] = value
	}
	return output, nil
}

func (state *schemaLocationNormalizer) reference(reference string) (string, error) {
	if !strings.HasPrefix(reference, "#") {
		return reference, nil
	}
	pointer, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return "", fmt.Errorf("schema reference %q has an invalid URI fragment escape", reference)
	}
	if !strings.HasPrefix(pointer, "/") {
		return reference, nil // unresolved anchors are diagnosed by the resolver
	}
	resolved, err := resolveSchemaLocationPointer(state.root, pointer)
	if err != nil {
		return "", fmt.Errorf("schema reference %q: %w", reference, err)
	}
	canonical := schemaLocalReference(pointer)
	if strings.HasPrefix(pointer, "/components/schemas/") && len(strings.Split(pointer, "/")) == 4 {
		return canonical, nil
	}
	name, exists := state.names[canonical]
	if !exists {
		name = fmt.Sprintf("Ref_%x", sha256.Sum256([]byte(canonical)))
		for state.used[name] {
			name += "_"
		}
		state.used[name] = true
		state.names[canonical] = name
		state.aliases[name] = resolved
		state.paths[name] = pointer
	}
	return schemaLocalReference(appendSchemaPointer("/components/schemas", name)), nil
}

func (state *schemaLocationNormalizer) rewrite(value any, pointer string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		tokens, tokenErr := schemaPointerTokens(pointer)
		candidate := tokenErr == nil && openapiwalk.ObjectContextAt(tokens) == openapiwalk.ObjectSchema
		result := make(map[string]any, len(typed))
		for _, key := range sortedOpaqueMapKeys(typed) {
			child := typed[key]
			if candidate {
				if key == "$ref" {
					if reference, ok := child.(string); ok {
						normalized, err := state.reference(reference)
						if err != nil {
							return nil, err
						}
						result[key] = normalized
						continue
					}
				}
				if key == dynamicReferenceMetadataKey {
					metadata, _ := child.(map[string]any)
					if reference, ok := metadata["reference"].(string); ok {
						normalized, err := state.reference(reference)
						if err != nil {
							return nil, err
						}
						result[key] = map[string]any{"anchor": metadata["anchor"], "reference": normalized}
						continue
					}
				}
			}
			if tokenErr != nil || openapiwalk.ReferenceChildOpaque(tokens, key, child) {
				result[key] = child
				continue
			}
			normalized, err := state.rewrite(child, appendSchemaPointer(pointer, key))
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			normalized, err := state.rewrite(child, appendSchemaPointer(pointer, strconv.Itoa(index)))
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
}

func schemaReferenceLiteralKey(key string) bool {
	switch key {
	case "const", "default", "enum", "example", "examples", "value", "dataValue", "serializedValue":
		return true
	default:
		return false
	}
}

func resolveSchemaLocationPointer(root map[string]any, pointer string) (any, error) {
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	var value any = root
	for _, token := range tokens {
		decoded, err := decodeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		switch current := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = current[decoded]
			if !exists {
				return nil, fmt.Errorf("unresolved Schema location %q", pointer)
			}
		case []any:
			index, err := strconv.Atoi(decoded)
			if err != nil || index < 0 || index >= len(current) || strconv.Itoa(index) != decoded {
				return nil, fmt.Errorf("invalid Schema array index %q", decoded)
			}
			value = current[index]
		default:
			return nil, fmt.Errorf("unresolved Schema location %q", pointer)
		}
	}
	if schemaPointerContext(pointer) != openapiwalk.ObjectSchema {
		return nil, fmt.Errorf("%q does not target a Schema Object location", pointer)
	}
	if _, ok := value.(map[string]any); !ok {
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("%q does not target a Schema Object", pointer)
		}
	}
	return value, nil
}

func decodeJSONPointerToken(token string) (string, error) {
	// Most document path tokens have no escape. Share their existing bytes
	// instead of copying every token at each reference-normalization visit.
	if !strings.ContainsRune(token, '~') {
		return token, nil
	}
	var result strings.Builder
	result.Grow(len(token))
	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			result.WriteByte(token[index])
			continue
		}
		if index+1 >= len(token) {
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
		index++
		switch token[index] {
		case '0':
			result.WriteByte('~')
		case '1':
			result.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
	}
	return result.String(), nil
}
