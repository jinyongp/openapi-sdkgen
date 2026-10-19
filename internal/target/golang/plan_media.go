package golang

import (
	"fmt"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func isXMLMedia(value string) bool {
	value = bareMediaType(value)
	return value == "application/xml" || value == "text/xml" || strings.HasSuffix(value, "+xml")
}

func mediaMayContainMultipart(value string) bool {
	bare := bareMediaType(value)
	if strings.HasPrefix(strings.ToLower(value), "multipart/") {
		return true
	}
	major, _, _ := strings.Cut(bare, "/")
	return major == "*" && !strings.HasSuffix(bare, "+json") && !strings.HasSuffix(bare, "+xml")
}

func prepareBodyEncodings(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner) (string, error) {
	if operation.RequestBody == nil {
		return "", nil
	}
	var body strings.Builder
	fmt.Fprintf(&body, "func bodyEncodingFor%s(execution *core.Execution,media string)[]core.FormField{switch media{\n", planned.Name)
	for _, media := range operation.RequestBody.Content {
		identity := operation.RequestBody.Pointer + "/content/" + pointerToken(media.ContentType)
		definitions, err := prepareEncodingFields(planned, media.Raw, media.Schema, document, schemas, identity, mediaMayContainMultipart(media.ContentType), strings.HasPrefix(strings.ToLower(media.ContentType), "multipart/"))
		if err != nil {
			return "", err
		}
		if definitions != "nil" {
			fmt.Fprintf(&body, "case %q:return %s\n", media.ContentType, definitions)
		}
	}
	body.WriteString("};return nil}\n")
	fmt.Fprintf(&body, "func bodyPositionalFor%s(media string)bool{switch media{", planned.Name)
	for _, media := range operation.RequestBody.Content {
		_, prefix := media.Raw["prefixEncoding"]
		_, item := media.Raw["itemEncoding"]
		if prefix || item {
			fmt.Fprintf(&body, "case %q:return true;", media.ContentType)
		}
	}
	body.WriteString("};return false}\n")
	fmt.Fprintf(&body, "func itemEncodingFor%s(execution *core.Execution,media string)[]core.FormField{switch media{", planned.Name)
	for _, media := range operation.RequestBody.Content {
		if _, exists := media.Raw["itemSchema"]; !exists || !strings.HasPrefix(strings.ToLower(media.ContentType), "multipart/") {
			continue
		}
		raw := make(map[string]any)
		if value, exists := media.Raw["itemEncoding"]; exists {
			raw["itemEncoding"] = value
		}
		root := map[string]any{"type": "array", "items": media.ItemSchema}
		definitions, err := prepareEncodingFields(planned, raw, root, document, schemas, operation.RequestBody.Pointer+"/content/"+pointerToken(media.ContentType)+"/incremental", true, true)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "case %q:return %s\n", media.ContentType, definitions)
	}
	body.WriteString("};return nil}\n")
	return body.String(), nil
}

func prepareEncodingFields(planned *operationPlan, raw map[string]any, bodySchema any, document *ir.Document, schemas *schemaPlanner, identity string, multipart, projectBinary bool) (string, error) {
	encoding := make(map[string]any)
	positional := make(map[string]bool)
	fieldNames := make(map[string]string)
	defaults := make(map[string]bool)
	if named, ok := raw["encoding"].(map[string]any); ok {
		for key, value := range named {
			encoding["name:"+key] = value
			fieldNames["name:"+key] = key
		}
	}
	if prefix, ok := raw["prefixEncoding"].([]any); ok {
		for index, value := range prefix {
			key := "position:" + strconv.Itoa(index)
			encoding[key] = value
			positional[key] = true
			fieldNames[key] = strconv.Itoa(index)
		}
	}
	if item, exists := raw["itemEncoding"]; exists {
		encoding["position:*"] = item
		positional["position:*"] = true
		fieldNames["position:*"] = "*"
		defaults["position:*"] = true
	}
	if multipart {
		encoding["named-default:"] = map[string]any{}
		defaults["named-default:"] = true
		names, count := httpplan.MultipartSchemaNames(document, bodySchema)
		for _, name := range names {
			key := "name:" + name
			if _, exists := encoding[key]; !exists {
				encoding[key] = map[string]any{}
				fieldNames[key] = name
			}
		}
		for index := 0; index < count; index++ {
			key := "position:" + strconv.Itoa(index)
			if _, exists := encoding[key]; !exists {
				encoding[key] = raw["itemEncoding"]
				fieldNames[key] = strconv.Itoa(index)
				positional[key] = true
			}
		}
		if _, exists := encoding["position:*"]; !exists {
			encoding["position:*"] = map[string]any{}
			fieldNames["position:*"] = "*"
			positional["position:*"] = true
			defaults["position:*"] = true
		}
	}
	if len(encoding) == 0 {
		return "nil", nil
	}
	queryValidation := ""
	if multipart {
		var err error
		querySchema := bodySchema
		if projectBinary {
			querySchema = httpplan.MultipartInputSchema(document, "multipart/form-data", bodySchema, raw, true)
		}
		queryValidation, err = schemas.prepare(headerValidationName(planned.Name+"MultipartPartQuery", identity), querySchema, schemaplan.Input, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: identity + "/schema"})
		if err != nil {
			return "", err
		}
		planned.MultipartParts = append(planned.MultipartParts, multipartPartPlan{Validation: queryValidation})
	}
	var body strings.Builder
	body.WriteString("[]core.FormField{")
	for _, key := range sortedKeys(encoding) {
		name := fieldNames[key]
		field, _ := encoding[key].(map[string]any)
		contentType, _ := field["contentType"].(string)
		explode := !multipart || contentType == ""
		if value, exists := field["explode"].(bool); exists {
			explode = value
		}
		partSchema, err := httpplan.MultipartPartSchema(document, bodySchema, name, positional[key])
		if err != nil {
			return "", err
		}
		binaryContentType := ""
		if binary, ok := httpplan.MultipartBinaryInput(document, partSchema, field); ok {
			binaryContentType = binary.ContentType
		}
		fmt.Fprintf(&body, "{Name:%s,ContentType:%s,DefaultContentType:%q,BinaryContentType:%q,Explode:%t,Positional:%t,Default:%t,Headers:[]core.MultipartHeader{", strconv.Quote(name), strconv.Quote(contentType), multipartDefaultContentType(partSchema), binaryContentType, explode, positional[key], defaults[key])
		headers, err := prepareMultipartHeaders(planned, field, document, schemas, identity+"/encoding/"+pointerToken(key))
		if err != nil {
			return "", err
		}
		body.WriteString(headers)
		body.WriteString("},Children:")
		children := "nil"
		if multipart && strings.HasPrefix(bareMediaType(contentType), "multipart/") {
			children, err = prepareEncodingFields(planned, field, partSchema, document, schemas, identity+"/encoding/"+pointerToken(key), multipart, projectBinary)
			if err != nil {
				return "", err
			}
		}
		body.WriteString(children)
		if multipart && children == "nil" {
			// Options can select nested multipart even without a declared nested
			// encoding. Reuse the root query, including its runtime property and
			// repeated-item path, rather than expanding default encodings forever.
			fmt.Fprintf(&body, ",Nested:func(parent,index string)[]core.FormField{return []core.FormField{{Positional:true,Default:true,MediaInfo:func(ctx context.Context,value types.Value,name,_ string,binaries map[string]bool)(string,bool,error){key:=name;if index!=\"\"{key=index+\"/\"+key};return schema.MultipartPartMedia__%s__(ctx,value,parent,key,%t,binaries)},Encoder:func(ctx context.Context,value types.Value,contentType,name,_ string)(types.Binary,error){key:=name;if index!=\"\"{key=index+\"/\"+key};encoded,err:=schema.EncodeXMLPartQuery__%s__(ctx,value,contentType,parent,key,%t);return types.BinaryFromBytes(encoded),err}}}}", queryValidation, positional[key], queryValidation, positional[key])
		}
		if multipart {
			fmt.Fprintf(&body, ",MediaInfo:func(ctx context.Context,value types.Value,name,index string,binaries map[string]bool)(string,bool,error){return schema.MultipartPartMedia__%s__(ctx,value,name,index,%t,binaries)}", queryValidation, positional[key])
		}
		if multipart && (isXMLMedia(contentType) || strings.Contains(contentType, "*") || contentType == "") {
			fmt.Fprintf(&body, ",Encoder:func(ctx context.Context,value types.Value,contentType,name,index string)(types.Binary,error){encoded,err:=schema.EncodeXMLPartQuery__%s__(ctx,value,contentType,name,index,%t);return types.BinaryFromBytes(encoded),err}", queryValidation, positional[key])
		}
		body.WriteString("},")
	}
	body.WriteString("}")
	return body.String(), nil
}

func multipartDefaultContentType(value any) string {
	schema, _ := value.(map[string]any)
	var types []string
	if kind, ok := schema["type"].(string); ok {
		types = append(types, kind)
	}
	if kinds, ok := schema["type"].([]any); ok {
		for _, kind := range kinds {
			if name, ok := kind.(string); ok {
				types = append(types, name)
			}
		}
	}
	for _, kind := range types {
		if kind == "object" || kind == "array" {
			return "application/json"
		}
	}
	for _, kind := range types {
		if kind == "string" {
			if _, encoded := schema["contentEncoding"]; encoded {
				return "application/octet-stream"
			}
			return "text/plain"
		}
	}
	for _, kind := range types {
		if kind == "number" || kind == "integer" || kind == "boolean" {
			return "text/plain"
		}
	}
	return "application/octet-stream"
}

// XML decoding uses the same direct schema programs as contentSchema, while
// each HTTP occurrence retains its own media root and public wrapper identity.
func prepareXMLResponses(operations []operationPlan, schemas *schemaPlanner) string {
	var prepared strings.Builder
	register := func(name, validation string) string {
		node := schemas.nodes[validation]
		if schemas.xmlRequests == nil {
			schemas.xmlRequests = make(map[*schemaplan.Node]string)
		}
		binding := schemas.xmlRequests[node]
		if binding == "" {
			binding = "decodeXMLHTTP__" + name + "__"
			schemas.xmlRequests[node] = binding
		}
		schemas.requireHelper("wire-xml")
		decoder := "DecodeXMLHTTP__" + name + "__"
		fmt.Fprintf(&prepared, "func %s(ctx context.Context,data []byte,contentType string)(wire.Value,error){node,err:=wire.ParseXML(ctx,wire.UTF8Text(data));if err!=nil{return wire.Value{},err};return %s(ctx,node,\"\",&xmlState{validation:&State{active:make(map[validationVisit]bool),MediaType:contentType}})}\n", decoder, binding)
		return decoder
	}
	for operationIndex := range operations {
		operation := &operations[operationIndex]
		for groupIndex := range operation.Headers {
			for fieldIndex := range operation.Headers[groupIndex].Fields {
				field := &operation.Headers[groupIndex].Fields[fieldIndex]
				if isXMLMedia(field.ContentType) {
					field.XMLDecoder = register(field.Validation, field.Validation)
				}
			}
		}
		for _, field := range operation.MultipartHeaders {
			if isXMLMedia(field.ContentType) {
				register(field.Validation, field.Validation)
			}
		}
		for _, validation := range operation.MultipartResponses {
			register(validation, validation)
		}
		for responseIndex := range operation.Responses {
			response := &operation.Responses[responseIndex]
			if response.ItemValidation != "" && response.Framing == string(ir.StreamFramingMultipart) {
				response.ItemXMLDecoder = register(response.ItemModel, response.ItemValidation)
			}
			if response.Kind != "xml" && !response.Open || response.Validation == "" {
				continue
			}
			response.Decoder = register(response.Model, response.Validation)
		}
	}
	return prepared.String()
}

func prepareMultipartHeaders(planned *operationPlan, field map[string]any, document *ir.Document, schemas *schemaPlanner, identity string) (string, error) {
	var body strings.Builder
	headers, _ := field["headers"].(map[string]any)
	for _, headerName := range sortedKeys(headers) {
		if strings.EqualFold(headerName, "Content-Type") {
			continue
		}
		header, _ := headers[headerName].(map[string]any)
		header, err := httpplan.ResolveComponentObject(document, header, "headers")
		if err != nil {
			return "", err
		}
		value, headerMedia, err := httpplan.HeaderSchema(document, header)
		if err != nil {
			return "", err
		}
		identity := identity + "/headers/" + pointerToken(headerName)
		validation, err := schemas.prepare(headerValidationName(planned.Name+"MultipartHeaders", identity), value, schemaplan.Output, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: identity + "/schema", MediaType: headerMedia})
		if err != nil {
			return "", err
		}
		required, _ := header["required"].(bool)
		headerExplode, _ := header["explode"].(bool)
		planned.MultipartHeaders = append(planned.MultipartHeaders, headerFieldPlan{Name: headerName, Validation: validation, ContentType: headerMedia, Required: required, Explode: headerExplode})
		fmt.Fprintf(&body, "{Name:%q,Required:%t,Decode:func(ctx context.Context,text string)(types.Value,error){", headerName, required)
		if headerMedia == "" {
			fmt.Fprintf(&body, "return schema.DecodeHeader%s(ctx,text,%t)", schemas.nodeNames[schemas.nodes[validation]], headerExplode)
		} else if isXMLMedia(headerMedia) {
			fmt.Fprintf(&body, "return schema.DecodeXMLHTTP__%s__(ctx,[]byte(text),%q)", validation, headerMedia)
		} else {
			fmt.Fprintf(&body, "value,err:=execution.DecodeHeaderContent(text,%q,ctx);if err!=nil{return types.Value{},err};", headerMedia)
			if strings.HasPrefix(bareMediaType(headerMedia), "text/") {
				fmt.Fprintf(&body, "if text,ok:=value.String();ok{return schema.DecodeHeader%s(ctx,text,%t)};", schemas.nodeNames[schemas.nodes[validation]], headerExplode)
			}
			fmt.Fprintf(&body, "if _,err:=schema.%s(ctx,value,\"\");err!=nil{return types.Value{},err};return value,nil", validation)
		}
		body.WriteString("}},")
	}
	return body.String(), nil
}
