package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func inboundProtocol(framing string) string {
	switch framing {
	case "line-delimited-json":
		return "stream.JSONProtocol{}"
	case "json-sequence":
		return "stream.JSONProtocol{Sequence:true}"
	case "sse":
		return "stream.SSEProtocol{}"
	}
	return "types.StreamProtocol[types.Value](nil)"
}

func inboundFieldType(field inputFieldPlan) string {
	if field.Open {
		return "types.MediaValue[models." + field.Type + "]"
	}
	if field.Incremental {
		return "types.Source[models." + field.Stream.ItemModel + "]"
	}
	if field.Binary {
		return "types.Binary"
	}
	return "models." + field.Type
}

func emitInboundBodyDecode(body *bytes.Buffer, config generator.GoOptions, imports map[string]bool, operationName string, field inputFieldPlan) {
	if field.Open {
		fmt.Fprintf(body, "var value types.MediaValue[models.%s];if runtime.BinaryMedia(contentType,%t){binary,err:=runtime.DecodeBinaryBody(execution,%t);if err!=nil{return runtime.Response{},err};data,_:=binary.Bytes();value=types.MediaValueFromBytes[models.%s](contentType,data)}else if runtime.MultipartMedia(contentType){raw,err:=runtime.DecodeMultipart(execution,contentType,shapes[%q],bodyEncodingFor%s(execution,%q),false,%t,func(name string)runtime.StreamPartDefinition{return namedPartFor%s(execution,%q,name)});if err!=nil{return runtime.Response{},err};if raw.Value.Kind()!=types.ValueNull{if _,err:=models.DecodeMedia%s(execution.Context,raw);err!=nil{return runtime.Response{},runtime.InputError(err)}};value=types.MediaValueFromMultipart(contentType,raw,models.DecodeMedia%s)}else{raw,err:=runtime.DecodeBody(execution,contentType,\"open\",shapes[%q],%s);if err!=nil{return runtime.Response{},err};if _,err:=models.Decode%s(execution.Context,raw);err!=nil{return runtime.Response{},runtime.InputError(err)};value=types.MediaValueFromValue(contentType,raw,models.Decode%s)}\n", field.Type, field.BinaryFallback, field.Required, field.Type, field.Name, operationName, field.MediaType, field.Required, operationName, field.MediaType, field.Type, field.Type, field.Name, field.Encoder, field.Type, field.Type)
		if !field.Required {
			fmt.Fprintln(body, "if native,ok:=value.AsMultipart();ok&&native.Value.Kind()==types.ValueNull{present=false}")
		}
		return
	}
	if field.Binary {
		fmt.Fprintf(body, "value,err:=runtime.DecodeBinaryBody(execution,%t);if err!=nil{return runtime.Response{},err}\n", field.Required)
		return
	}
	if field.Multipart && field.Stream == nil {
		fmt.Fprintf(body, "raw,err:=runtime.DecodeMultipart(execution,contentType,shapes[%q],bodyEncodingFor%s(execution,%q),bodyPositionalFor%s(%q),%t,func(name string)runtime.StreamPartDefinition{return namedPartFor%s(execution,%q,name)});if err!=nil{return runtime.Response{},err};", field.Name, operationName, field.MediaType, operationName, field.MediaType, field.Required, operationName, field.MediaType)
		if !field.Required {
			fmt.Fprintf(body, "var value models.%s;if raw.Value.Kind()==types.ValueNull{present=false}else{var err error;value,err=models.DecodeMedia%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)}}\n", field.Type, field.Type)
		} else {
			fmt.Fprintf(body, "value,err:=models.DecodeMedia%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)}\n", field.Type)
		}
		return
	}
	if field.Stream != nil {
		if field.Stream.Multipart {
			definition := fmt.Sprintf("func(index int)runtime.StreamPartDefinition{return %s(execution,index)}", field.Stream.Parts)
			if field.Incremental {
				fmt.Fprintf(body, "value,err:=runtime.DecodeMultipartSource(execution,contentType,%s,models.DecodeMedia%s,models.EncodeMedia%s,%t);if err!=nil{return runtime.Response{},err}\n", definition, field.Stream.ItemModel, field.Stream.ItemModel, field.Required)
			} else {
				fmt.Fprintf(body, "raw,err:=runtime.DecodeCompleteMultipart(execution,contentType,%s,func(_ context.Context,value types.Value)(types.MultipartData,error){return types.MultipartData{Value:value},nil},%t);if err!=nil{return runtime.Response{},err};value,err:=models.DecodeMedia%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)}\n", definition, field.Required, field.Type)
			}
			return
		}
		protocol := inboundProtocol(field.Stream.Framing)
		if strings.HasPrefix(protocol, "stream.") {
			imports[config.ImportPath+"/internal/stream"] = true
		}
		if field.Incremental {
			fmt.Fprintf(body, "value,err:=runtime.DecodeSource(execution,contentType,%s,models.Decode%s,func(ctx context.Context,item models.%s)(models.%s,error){_,err:=models.Encode%s(ctx,item);return item,err},%t);if err!=nil{return runtime.Response{},err}\n", protocol, field.Stream.ItemModel, field.Stream.ItemModel, field.Stream.ItemModel, field.Stream.ItemModel, field.Required)
		} else {
			fmt.Fprintf(body, "raw,err:=runtime.DecodeCompleteStream(execution,contentType,%s,%t);if err!=nil{return runtime.Response{},err};value,err:=models.Decode%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)}\n", protocol, field.Required, field.Type)
		}
		return
	}
	decoder := field.Encoder
	if decoder == "" {
		decoder = "nil"
	}
	fmt.Fprintf(body, "raw,err:=runtime.DecodeBody(execution,contentType,%q,shapes[%q],%s);if err!=nil{return runtime.Response{},err};value,err:=models.Decode%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)}\n", responseMediaKind(field.MediaType), field.Name, decoder, field.Type)
}

func appendInboundFiles(config generator.GoOptions, files []filePlan, entries []inboundOperationPlan, schemas *schemaPlanner) []filePlan {
	packages := make(map[string]bool)
	for _, entry := range entries {
		pkg := "webhooks"
		if entry.Definition.Kind == "callback" {
			pkg = "callbacks"
		}
		packages[pkg] = true
		files = append(files, inboundFile(config, pkg, entry, schemas))
	}
	for _, pkg := range sortedKeys(packages) {
		files = append(files, filePlan{Path: "server/" + pkg + "/options.go", Package: pkg, Template: "templates/prepared.go.tmpl", Imports: []importPlan{{Path: config.ImportPath + "/server/runtime"}}, Prepared: "type Options=runtime.Options\n"})
	}
	return files
}

func inboundFile(config generator.GoOptions, pkg string, entry inboundOperationPlan, schemas *schemaPlanner) filePlan {
	name, operation := entry.PublicName, entry.Operation
	var body bytes.Buffer
	imports := map[string]bool{"context": true, "fmt": true, "net/http": true, config.ImportPath + "/types": true, config.ImportPath + "/server/runtime": true}
	modelImport := func() { imports[config.ImportPath+"/models"] = true }
	roots := make(map[string]*schemaplan.Node)
	for _, field := range operation.InputFields {
		if field.Validation != "" && field.Stream == nil {
			roots[field.Name] = schemas.nodes[field.Validation]
		}
	}
	for _, variant := range operation.RequestBodies {
		if variant.Validation != "" && variant.Stream == nil {
			roots[variant.Name] = schemas.nodes[variant.Validation]
		}
	}

	headerRoots := make(map[string]*schemaplan.Node)
	for _, group := range operation.Headers {
		for _, field := range group.Fields {
			headerRoots[group.Status+":"+field.Name] = schemas.nodes[field.Validation]
		}
	}
	if len(headerRoots) > 0 {
		imports[config.ImportPath+"/internal/schema"] = true
		imports["sync"] = true
		fmt.Fprintln(&body, prepareParameterGraph("headerGraphFor"+name, headerRoots, schemas))
		fmt.Fprintf(&body, "var headerShapesFor%s=sync.OnceValue(headerGraphFor%s)\n", name, name)
	}
	if len(roots) != 0 {
		imports[config.ImportPath+"/internal/schema"] = true
		fmt.Fprintln(&body, prepareParameterGraph("parameterGraphFor"+name, roots, schemas))
	}
	if operation.BodyEncodings != "" {
		fmt.Fprintln(&body, operation.BodyEncodings)
		if len(operation.MultipartParts) != 0 || len(operation.MultipartHeaders) != 0 || len(operation.MultipartResponses) != 0 {
			imports[config.ImportPath+"/internal/schema"] = true
		}
	}
	fmt.Fprintf(&body, "type %sInput struct{Context runtime.InboundRequestContext\n", name)
	for _, field := range operation.InputFields {
		if field.Union {
			typ := name + "Body"
			if !field.Required {
				typ = "types.Optional[" + typ + "]"
			}
			fmt.Fprintf(&body, "Body %s\n", typ)
			continue
		}
		typ := inboundFieldType(field)
		if !field.Binary {
			modelImport()
		}
		if !field.Required {
			typ = "types.Optional[" + typ + "]"
		}
		fmt.Fprintf(&body, "%s %s\n", field.Name, typ)
	}
	fmt.Fprintln(&body, "}")
	if operation.BodyUnion {
		fmt.Fprintf(&body, "type %sBody struct{contentType string;value any}\nfunc(value %sBody)ContentType()string{return value.contentType}\n", name, name)
		for _, variant := range operation.RequestBodies {
			field := inputFieldPlan{Type: variant.Model, Open: variant.Open, Binary: variant.Binary, Incremental: variant.Incremental, Stream: variant.Stream}
			typ := inboundFieldType(field)
			if !variant.Binary {
				modelImport()
			}
			fmt.Fprintf(&body, "func(value %sBody)As%s()(%s,bool){result,ok:=value.value.(%s);return result,ok}\n", name, variant.Name, typ, typ)
		}
	}
	fmt.Fprintf(&body, "type %sHandler func(context.Context,%sInput)(%sResponse,error)\ntype %sResponse struct{build func(*runtime.Execution,http.Header)(runtime.Response,error);headers http.Header;multipart runtime.MultipartOptions}\n", name, name, name, name)
	fmt.Fprintf(&body, "func(value %sResponse)WithHeaders(headers http.Header)%sResponse{value.headers=headers.Clone();return value}\n", name, name)
	fmt.Fprintf(&body, "func(value %sResponse)WithMultipartOptions(options runtime.MultipartOptions)%sResponse{value.multipart=runtime.CopyMultipartOptions(options);return value}\n", name, name)
	for _, group := range operation.Headers {
		if group.Model == "" {
			continue
		}
		modelImport()
		for _, header := range group.Fields {
			if isXMLMedia(header.ContentType) {
				imports[config.ImportPath+"/internal/schema"] = true
			}
		}
		emitInboundHeaders(&body, entry, group)
	}
	statuses := make([]string, len(operation.Responses))
	for index, response := range operation.Responses {
		statuses[index] = strconv.Quote(response.Status)
	}
	for _, response := range operation.Responses {
		headerGroup := inboundHeaderGroup(operation, response.Status)
		encoding := "nil"
		if helper := entry.ResponseEncodings[response.Status+"\x00"+response.MediaType]; helper != "" {
			encoding = helper + "(execution)"
		}
		status, err := strconv.Atoi(response.Status)
		statusParameter, statusValue := "", strconv.Itoa(status)
		if err != nil {
			statusParameter, statusValue = "status int", "status"
		}
		parameters := []string{}
		if statusParameter != "" {
			parameters = append(parameters, statusParameter)
		}
		hasBody := response.Kind != "none"
		typ := response.Type
		if response.Kind == "binary" {
			typ = "types.Binary"
		} else if response.Kind == "stream" {
			typ = "types.Binary"
			if response.ItemValidation != "" {
				typ = "types.Source[models." + response.ItemModel + "]"
			}
		}
		if hasBody {
			parameters = append(parameters, "value "+typ)
			if response.Kind != "binary" && !(response.Kind == "stream" && response.ItemValidation == "") {
				modelImport()
			}
		}
		if headerGroup.Model != "" {
			parameters = append(parameters, "typedHeaders models."+headerGroup.Model)
		}
		fmt.Fprintf(&body, "func New%s%sResponse(%s)%sResponse{return %sResponse{build:func(execution *runtime.Execution,headers http.Header)(runtime.Response,error){\nstatus:=%s\n", name, response.Name, strings.Join(parameters, ","), name, name, statusValue)
		if headerGroup.Model != "" {
			fmt.Fprintf(&body, "preparedHeaders,err:=encode%s%sHeaders(execution,typedHeaders,headers);if err!=nil{return runtime.Response{},err};headers=preparedHeaders\n", name, headerGroup.Name)
		}
		fmt.Fprintf(&body, "if !runtime.ResponseStatus(status,%q,[]string{%s}){return runtime.Response{},fmt.Errorf(\"response status does not match declared variant\")}\n", response.Status, strings.Join(statuses, ","))
		if !hasBody {
			fmt.Fprintln(&body, "return runtime.Response{Status:status,Headers:headers},nil}}}")
			continue
		}
		switch response.Kind {
		case "binary":
			fmt.Fprintf(&body, "encoded:=runtime.BinaryContentType(value,%q)\n", response.MediaType)
		case "stream":
			if response.ItemValidation == "" {
				fmt.Fprintf(&body, "encoded:=runtime.BinaryContentType(value,%q)\n", response.MediaType)
				break
			}
			if response.Framing == "multipart" {
				fmt.Fprintf(&body, "encoded,err:=runtime.EncodeMultipartSource(execution,%q,value,models.EncodeMedia%s,%s);if err!=nil{return runtime.Response{},err}\n", response.MediaType, response.ItemModel, encoding)
				break
			}
			protocol := inboundProtocol(response.Framing)
			if strings.HasPrefix(protocol, "stream.") {
				imports[config.ImportPath+"/internal/stream"] = true
			}
			fmt.Fprintf(&body, "encoded,err:=runtime.EncodeSource(execution,%q,%s,value,models.Encode%s,func(ctx context.Context,item models.%s)(models.%s,error){_,err:=models.Encode%s(ctx,item);return item,err});if err!=nil{return runtime.Response{},err}\n", response.MediaType, protocol, response.ItemModel, response.ItemModel, response.ItemModel, response.ItemModel)
		default:
			if response.Open {
				encoder := inboundXMLEncoder(entry, "response:"+response.Name)
				if encoder != "nil" {
					imports[config.ImportPath+"/internal/schema"] = true
				}
				fmt.Fprintf(&body, "contentType:=value.ContentType();if !runtime.MediaMatches(contentType,%q){return runtime.Response{},fmt.Errorf(\"response media does not match declared variant\")};var encoded types.Binary;if data,present:=value.AsBinary();present{if !runtime.BinaryMedia(contentType,%t){return runtime.Response{},fmt.Errorf(\"structured media requires a validated value\")};encoded=runtime.BinaryContentType(types.BinaryFromBytes(data),contentType)}else if native,present:=value.AsMultipart();present{if !runtime.MultipartMedia(contentType){return runtime.Response{},fmt.Errorf(\"native multipart requires multipart media\")};typed,err:=models.DecodeMedia%s(execution.Context,native);if err!=nil{return runtime.Response{},err};logical,err:=models.EncodeMedia%s(execution.Context,typed);if err!=nil{return runtime.Response{},err};encoded,err=runtime.EncodeMultipartBody(execution,contentType,logical,%s,false);if err!=nil{return runtime.Response{},err}}else{typed,present:=value.AsValue();if !present{return runtime.Response{},fmt.Errorf(\"response media value is invalid\")};logical,err:=models.Encode%s(execution.Context,typed);if err!=nil{return runtime.Response{},err};encoded,err=runtime.EncodeBody(execution,logical,contentType,\"open\",%s,nil);if err!=nil{return runtime.Response{},err}}\n", response.MediaType, response.BinaryFallback, response.Model, response.Model, encoding, response.Model, encoder)
				break
			}
			if strings.HasPrefix(strings.ToLower(response.MediaType), "multipart/") {
				fmt.Fprintf(&body, "logical,err:=models.EncodeMedia%s(execution.Context,value);if err!=nil{return runtime.Response{},err};encoded,err:=runtime.EncodeMultipartBody(execution,%q,logical,%s,false);if err!=nil{return runtime.Response{},err}\n", response.Model, response.MediaType, encoding)
				break
			}
			encoder := inboundXMLEncoder(entry, "response:"+response.Name)
			if encoder != "nil" {
				imports[config.ImportPath+"/internal/schema"] = true
			}
			fmt.Fprintf(&body, "logical,err:=models.Encode%s(execution.Context,value);if err!=nil{return runtime.Response{},err};encoded,err:=runtime.EncodeBody(execution,logical,%q,%q,%s,%s);if err!=nil{return runtime.Response{},err}\n", response.Model, response.MediaType, response.Kind, encoder, encoding)
		}
		fmt.Fprintf(&body, "return runtime.Response{Status:status,Headers:headers,Body:encoded,HasBody:true,Streaming:%t},nil}}}\n", response.Kind == "stream")
		if response.Open {
			parameters := []string{}
			if statusParameter != "" {
				parameters = append(parameters, statusParameter)
			}
			parameters = append(parameters, "contentType string", "value types.Binary")
			if headerGroup.Model != "" {
				parameters = append(parameters, "typedHeaders models."+headerGroup.Model)
			}
			fmt.Fprintf(&body, "func New%s%sBinaryResponse(%s)%sResponse{return %sResponse{build:func(execution *runtime.Execution,headers http.Header)(runtime.Response,error){status:=%s;if !runtime.ResponseStatus(status,%q,[]string{%s})||!runtime.MediaMatches(contentType,%q)||!runtime.BinaryMedia(contentType,%t){return runtime.Response{},fmt.Errorf(\"binary response does not match declared variant\")};", name, response.Name, strings.Join(parameters, ","), name, name, statusValue, response.Status, strings.Join(statuses, ","), response.MediaType, response.BinaryFallback)
			if headerGroup.Model != "" {
				fmt.Fprintf(&body, "preparedHeaders,err:=encode%s%sHeaders(execution,typedHeaders,headers);if err!=nil{return runtime.Response{},err};headers=preparedHeaders;", name, headerGroup.Name)
			}
			fmt.Fprintln(&body, "return runtime.Response{Status:status,Headers:headers,Body:runtime.BinaryContentType(value,contentType),HasBody:true},nil}}}")
		}
		if response.Kind == "stream" && response.Buffered {
			parameters := []string{}
			if statusParameter != "" {
				parameters = append(parameters, statusParameter)
			}
			parameters = append(parameters, "value models."+response.Model)
			if headerGroup.Model != "" {
				parameters = append(parameters, "typedHeaders models."+headerGroup.Model)
			}
			fmt.Fprintf(&body, "func New%s%sCompleteResponse(%s)%sResponse{return %sResponse{build:func(execution *runtime.Execution,headers http.Header)(runtime.Response,error){", name, response.Name, strings.Join(parameters, ","), name, name)
			fmt.Fprintf(&body, "status:=%s;if !runtime.ResponseStatus(status,%q,[]string{%s}){return runtime.Response{},fmt.Errorf(\"response status does not match declared variant\")};", statusValue, response.Status, strings.Join(statuses, ","))
			if headerGroup.Model != "" {
				fmt.Fprintf(&body, "preparedHeaders,err:=encode%s%sHeaders(execution,typedHeaders,headers);if err!=nil{return runtime.Response{},err};headers=preparedHeaders;", name, headerGroup.Name)
			}
			if response.Framing == "multipart" {
				fmt.Fprintf(&body, "logical,err:=models.EncodeMedia%s(execution.Context,value);if err!=nil{return runtime.Response{},err};encoded,err:=runtime.EncodeMultipartBody(execution,%q,logical,%s,true);if err!=nil{return runtime.Response{},err};return runtime.Response{Status:%s,Headers:headers,Body:encoded,HasBody:true,Streaming:true},nil}}}\n", response.Model, response.MediaType, encoding, statusValue)
			} else {
				fmt.Fprintf(&body, "logical,err:=models.Encode%s(execution.Context,value);if err!=nil{return runtime.Response{},err};encoded,err:=runtime.EncodeCompleteStream(execution,%q,%s,logical);if err!=nil{return runtime.Response{},err};return runtime.Response{Status:%s,Headers:headers,Body:encoded,HasBody:true,Streaming:true},nil}}}\n", response.Model, response.MediaType, inboundProtocol(response.Framing), statusValue)
			}
		}
	}
	// Security requirements retain their original URI keys independently of the
	// canonical scheme names used by the credential candidates.
	requirements := make([]any, len(entry.Definition.Operation.Security))
	for index, requirement := range entry.Definition.Operation.Security {
		requirements[index] = requirement.Raw
	}
	securityJSON, _ := json.Marshal(requirements)
	fmt.Fprintf(&body, "func securityMetadataFor%s(value string)types.Value{result,_:=types.ParseValue([]byte(value));return result}\n", operation.Name)
	fmt.Fprintf(&body, "%sfunc New%sHandler(path string,options Options,handler %sHandler)(http.Handler,error){\nif handler==nil{return nil,fmt.Errorf(\"inbound handler is nil\")}\n", operation.Documentation, name, name)
	if len(roots) != 0 {
		fmt.Fprintf(&body, "shapes:=parameterGraphFor%s()\n", name)
	}
	fmt.Fprintf(&body, "security:=securityMetadataFor%s(%q)\nschemes:=[]runtime.InboundScheme{}\n", operation.Name, string(securityJSON))
	for _, requirement := range operation.Security {
		fmt.Fprintf(&body, "for _,scheme:=range (types.SecurityRequirementDefinition%s).Schemes{schemes=append(schemes,runtime.InboundScheme{Identity:scheme.Name,Definition:scheme})}\n", requirement.Definition)
	}
	fmt.Fprintf(&body, "definition:=runtime.Definition{OperationID:%q,Method:%q,Identity:%q,Security:security,Schemes:schemes}\n", operation.OperationID, operation.Method, entry.Definition.Identity)
	for _, field := range operation.InputFields {
		if field.Incremental {
			fmt.Fprintf(&body, "definition.Streaming=func(contentType string)bool{return runtime.MediaMatches(contentType,%q)}\n", field.MediaType)
		}
	}
	for _, variant := range operation.RequestBodies {
		if variant.Incremental {
			fmt.Fprintf(&body, "{previous:=definition.Streaming;definition.Streaming=func(contentType string)bool{return runtime.MediaMatches(contentType,%q)||previous!=nil&&previous(contentType)}}\n", variant.MediaType)
		}
	}
	parameters := entry.Definition.Operation.Parameters
	if len(parameters) != 0 {
		fmt.Fprintln(&body, "definitions:=[]runtime.Parameter{")
		for _, parameter := range parameters {
			decoder := "nil"
			if xml := inboundXMLDecoder(entry, "parameter:"+parameter.Location+":"+parameter.Name); xml != "nil" {
				decoder = fmt.Sprintf("func(ctx context.Context,text string)(types.Value,error){return %s(ctx,[]byte(text),%q)}", xml, parameter.ContentType)
			}
			fmt.Fprintf(&body, "{Name:%q,Location:%q,Style:%q,ContentType:%q,Explode:%t,Required:%t,AllowEmptyValue:%t,Schema:shapes[%q].Properties[%q],DecodeContent:%s},\n", parameter.Name, parameter.Location, parameter.Style, parameter.ContentType, parameter.Explode, parameter.Required, parameter.AllowEmptyValue, exportedName(parameter.Location), parameter.Name, decoder)
		}
		fmt.Fprintln(&body, "}\ndefinition.DecodeParameters=func(execution *runtime.Execution,path map[string]string)(runtime.ParameterResult,error){value,err:=runtime.DecodeParameters(execution,path,definitions);return runtime.ParameterResult{Value:value},err}")
	}
	fmt.Fprintf(&body, "return runtime.NewHandler(path,definition,options,func(execution *runtime.Execution,parameters runtime.ParameterResult)(runtime.Response,error){\ninput:=%sInput{Context:execution.InboundContext()}\n", name)
	for _, field := range operation.InputFields {
		if field.Name == "Body" {
			continue
		}
		fmt.Fprintf(&body, "{raw,_:=parameters.Value.Get(%q);", strings.ToLower(field.Name))
		if !field.Required {
			fmt.Fprint(&body, "if raw.Len()!=0{")
		}
		fmt.Fprintf(&body, "value,err:=models.Decode%s(execution.Context,raw);if err!=nil{return runtime.Response{},runtime.InputError(err)};", field.Type)
		if field.Required {
			fmt.Fprintf(&body, "input.%s=value", field.Name)
		} else {
			fmt.Fprintf(&body, "input.%s=types.Some(value)}", field.Name)
		}
		fmt.Fprintln(&body, "}")
	}
	for _, field := range operation.InputFields {
		if field.Name != "Body" {
			continue
		}
		if field.Union {
			fmt.Fprintln(&body, "{contentType:=execution.Request.Header.Get(\"Content-Type\");")
			if !field.Required {
				fmt.Fprintln(&body, "if contentType!=\"\"||execution.HasBody(){")
			}
			media := make([]string, len(operation.RequestBodies))
			for index, variant := range operation.RequestBodies {
				media[index] = strconv.Quote(variant.MediaType)
			}
			fmt.Fprintf(&body, "switch runtime.SelectMedia(contentType,[]string{%s}){\n", strings.Join(media, ","))
			for index, variant := range operation.RequestBodies {
				fmt.Fprintf(&body, "case %d:\n", index)
				variantField := inputFieldPlan{Required: field.Required, Name: variant.Name, Type: variant.Model, Validation: variant.Validation, Open: variant.Open, BinaryFallback: variant.BinaryFallback, Binary: variant.Binary, Multipart: variant.Multipart, Incremental: variant.Incremental, Stream: variant.Stream, MediaType: variant.MediaType}
				variantField.Encoder = inboundXMLDecoder(entry, "body:"+variant.Name)
				closePresence := emitInboundBodyPresence(&body, variantField)
				emitInboundBodyDecode(&body, config, imports, operation.Name, variantField)
				if field.Required {
					fmt.Fprintf(&body, "input.Body=%sBody{contentType:contentType,value:value}\n", name)
				} else {
					if variantField.Open || variantField.Multipart && variantField.Stream == nil {
						fmt.Fprintln(&body, "if present{")
					}
					fmt.Fprintf(&body, "input.Body=types.Some(%sBody{contentType:contentType,value:value})\n", name)
					if variantField.Open || variantField.Multipart && variantField.Stream == nil {
						fmt.Fprintln(&body, "}")
					}
				}
				if closePresence {
					fmt.Fprintln(&body, "}")
				}
			}
			fmt.Fprintln(&body, "default:return runtime.Response{},runtime.InputError(&runtime.Error{Status:415,Code:runtime.InboundInputInvalid})\n}")
			if !field.Required {
				fmt.Fprintln(&body, "}")
			}
			fmt.Fprintln(&body, "}")
			continue
		}
		fmt.Fprintln(&body, "{contentType:=execution.Request.Header.Get(\"Content-Type\");")
		if !field.Required {
			fmt.Fprintln(&body, "if contentType!=\"\"||execution.HasBody(){")
		}
		fmt.Fprintf(&body, "if !runtime.MediaMatches(contentType,%q){return runtime.Response{},runtime.InputError(&runtime.Error{Status:415,Code:runtime.InboundInputInvalid})}\n", field.MediaType)
		field.Encoder = inboundXMLDecoder(entry, "body:"+field.Name)
		closePresence := emitInboundBodyPresence(&body, field)
		emitInboundBodyDecode(&body, config, imports, operation.Name, field)
		if field.Required {
			fmt.Fprintln(&body, "input.Body=value")
		} else {
			if field.Multipart && field.Stream == nil {
				fmt.Fprintln(&body, "if present{")
			}
			fmt.Fprintln(&body, "input.Body=types.Some(value)")
			if field.Multipart && field.Stream == nil {
				fmt.Fprintln(&body, "}")
			}
		}
		if closePresence {
			fmt.Fprintln(&body, "}")
		}
		if !field.Required {
			fmt.Fprintln(&body, "}")
		}
		fmt.Fprintln(&body, "}")
	}
	fmt.Fprintln(&body, "response,err:=handler(execution.Context,input);if err!=nil{return runtime.Response{},err};if response.build==nil{return runtime.Response{},runtime.ResponseError(fmt.Errorf(\"response variant is missing\"))};execution.ResponseMultipart=response.multipart;result,err:=response.build(execution,response.headers.Clone());if err!=nil{return runtime.Response{},runtime.ResponseError(err)};return result,nil\n})}")
	definition := entry.Definition
	fmt.Fprintf(&body, "func %sInfo()struct{Name,Identity,Expression,SourceRoute,SourceOperationID string}{return struct{Name,Identity,Expression,SourceRoute,SourceOperationID string}{%q,%q,%q,%q,%q}}\n", name, definition.Name, definition.Identity, definition.Expression, definition.SourceRoute, definition.SourceOperationID)
	file := filePlan{Path: sourceFilePath("server/"+pkg, name), Package: pkg, Template: "templates/prepared.go.tmpl", Prepared: body.String()}
	for _, imported := range sortedKeys(imports) {
		file.Imports = append(file.Imports, importPlan{Path: imported})
	}
	if operation.BodyEncodings != "" {
		file.Imports = append(file.Imports, importPlan{Alias: "core", Path: config.ImportPath + "/server/runtime"})
	}
	return file
}
