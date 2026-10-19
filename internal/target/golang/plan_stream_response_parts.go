package golang

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// Functions select direct, compiled part contracts. Caching before descent also
// represents recursive array schemas without expanding them at generation time.
type responsePartPlanner struct {
	operation  *operationPlan
	document   *ir.Document
	schemas    *schemaPlanner
	names      map[string]string
	functions  []string
	projection schemaplan.Projection
	prefix     string
}

func newResponsePartPlanner(operation *operationPlan, document *ir.Document, schemas *schemaPlanner) *responsePartPlanner {
	return &responsePartPlanner{operation: operation, document: document, schemas: schemas, names: make(map[string]string)}
}
func (p *responsePartPlanner) source() string { return strings.Join(p.functions, "\n") }
func (p *responsePartPlanner) prepare(root any, raw map[string]any) (string, error) {
	if root == nil {
		root = true
	}
	// Only the encoding children affect nested representation selection.
	encoding := map[string]any{"prefixEncoding": raw["prefixEncoding"], "itemEncoding": raw["itemEncoding"]}
	key, err := json.Marshal([]any{root, encoding})
	if err != nil {
		return "", err
	}
	if name := p.names[string(key)]; name != "" {
		return name, nil
	}
	helperPrefix := p.prefix
	if helperPrefix == "" {
		helperPrefix = "responsePartsFor"
	}
	name := fmt.Sprintf("%s%s%d", helperPrefix, p.operation.Name, len(p.names)+1)
	p.names[string(key)] = name
	// Response framing reads the sequence's own facets, as the existing native
	// TypeScript decoder does. Complete schema assertions still run afterward.
	// Projecting arbitrary combinators here can grow a recursive schema forever.
	sequence, _ := root.(map[string]any)
	prefixSchemas, _ := sequence["prefixItems"].([]any)
	count := len(prefixSchemas)
	prefix, _ := raw["prefixEncoding"].([]any)
	count = max(count, len(prefix))
	var body strings.Builder
	fmt.Fprintf(&body, "func %s(execution *core.Execution,index int)core.StreamPartDefinition{switch index{", name)
	for index := 0; index <= count; index++ {
		partName := strconv.Itoa(index)
		field, _ := raw["itemEncoding"].(map[string]any)
		if index < count {
			fmt.Fprintf(&body, "case %d:", index)
			if index < len(prefix) {
				field, _ = prefix[index].(map[string]any)
			}
		} else {
			body.WriteString("default:")
			partName = "*"
		}
		partSchema := sequence["items"]
		if index < len(prefixSchemas) {
			partSchema = prefixSchemas[index]
		}
		if partSchema == nil {
			partSchema = true
		}
		identity := name + "/" + partName
		projection := p.projection
		if projection == "" {
			projection = schemaplan.Output
		}
		validation, err := p.schemas.prepare(headerValidationName(p.operation.Name+"ResponsePart", identity), partSchema, projection, p.document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: identity})
		if err != nil {
			return "", err
		}
		p.operation.MultipartResponses = append(p.operation.MultipartResponses, validation)
		headers, err := prepareMultipartHeaders(p.operation, field, p.document, p.schemas, identity)
		if err != nil {
			return "", err
		}
		child, err := p.prepare(partSchema, field)
		if err != nil {
			return "", err
		}
		contentType, _ := field["contentType"].(string)
		contentType, _, _ = strings.Cut(contentType, ",")
		contentType = strings.TrimSpace(contentType)
		partObject, _ := partSchema.(map[string]any)
		encoding, _ := partObject["contentEncoding"].(string)
		binary := encoding == "binary"
		fmt.Fprintf(&body, "return core.StreamPartDefinition{ContentType:%q,Binary:%t,Headers:[]core.MultipartHeader{%s},XML:schema.DecodeXMLHTTP__%s__,Nested:func(index int)core.StreamPartDefinition{return %s(execution,index)}};", contentType, binary, headers, validation, child)
	}
	body.WriteString("}}\n")
	p.functions = append(p.functions, body.String())
	return name, nil
}
