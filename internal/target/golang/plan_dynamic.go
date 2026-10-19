package golang

import (
	"bytes"
	"fmt"
	"openapi-sdkgen/internal/generator"
	"strings"
)

func appendDynamicFiles(config generator.GoOptions, files []filePlan, operations []operationPlan, links []linkSourcePlan) []filePlan {
	if len(operations) == 0 {
		return files
	}
	files = append(files, filePlan{Path: "internal/operations/dynamic.go", Package: "operations", Template: "runtime/dynamic-operation.go.tmpl", Imports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "net/http"}, {Path: "strconv"}, {Path: "strings"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/types"}}})
	for _, operation := range operations {
		var output bytes.Buffer
		emitDynamicInput(&output, operation)
		emitDynamicTerminals(&output, operation)
		info := fmt.Sprintf("types.OperationInfo{OperationID:%q,Route:%q,Method:%q,Path:%q}", operation.OperationID, operation.Route, operation.Method, operation.Path)
		fmt.Fprintf(&output, "func NewDynamic%s(client *core.Client)DynamicOperation{return DynamicOperation{info:%s,buffered:%t,streaming:%t,", operation.Name, info, operation.CanDo, operation.HasTypedStream)
		if operation.LinkSource {
			fmt.Fprintf(&output, "Links:dynamicLinksFor%s(client),", operation.Name)
		}
		fmt.Fprintf(&output, "call:func(value types.Value)DynamicCall{return newDynamic%sData(client,core.LinkData{Value:value})},native:func(data types.NativeInput)DynamicCall{return newDynamic%sData(client,data)}}}\n", operation.Name, operation.Name)
		fmt.Fprintf(&output, "func newDynamic%sData(client *core.Client,data core.LinkData)DynamicCall{info:=%s;build:=func(ctx context.Context,options types.RequestOptions)(%sCall,error){call,err:=dynamicInputFor%s(ctx,client,data);if err!=nil{return call,core.RequestFailure(ctx,info,types.RequestEncodeFailed,err,nil,nil)};return call.WithOptions(options),nil};return DynamicCall{", operation.Name, info, operation.Name, operation.Name)
		if operation.CanDo {
			fmt.Fprintf(&output, "do:func(ctx context.Context,options types.RequestOptions)(types.Value,error){call,err:=build(ctx,options);if err!=nil{return types.Value{},err};return call.dynamicDo(ctx)},")
		}
		fmt.Fprintf(&output, "raw:func(ctx context.Context,options types.RequestOptions)(*DynamicRawResponse,error){call,err:=build(ctx,options);if err!=nil{return nil,err};return call.dynamicRaw(ctx)},")
		if operation.HasTypedStream {
			fmt.Fprint(&output, "stream:func(ctx context.Context,options types.RequestOptions)*types.Stream[types.Value]{return types.NewStream(ctx,func(running context.Context)(types.Source[types.Value],*http.Response,error){call,err:=build(running,options);if err!=nil{return nil,nil,err};source:=call.dynamicStream(running);response,err:=source.Response();return source,response,err})},")
		}
		fmt.Fprintln(&output, "}}")
		for _, source := range links {
			if source.Operation.Route == operation.Route {
				emitDynamicLinks(&output, source)
			}
		}
		imports := []importPlan{{Path: "context"}, {Path: "net/http"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/types"}}
		if strings.Contains(output.String(), "models.") {
			imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
		}
		if strings.Contains(output.String(), "schema.") {
			imports = append(imports, importPlan{Path: config.ImportPath + "/internal/schema"})
		}
		if strings.Contains(output.String(), "strings.") {
			imports = append(imports, importPlan{Path: "strings"})
		}
		files = append(files, filePlan{Path: namedSourceFilePath("internal/operations", "dynamic", operation.Name), Package: "operations", Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: output.String()})
	}
	return files
}

func emitDynamicInput(output *bytes.Buffer, operation operationPlan) {
	fmt.Fprintf(output, "func dynamicInputFor%s(ctx context.Context,client *core.Client,data core.LinkData)(call %sCall,err error){call.client=client;value:=data.Value;_=value;", operation.Name, operation.Name)
	fmt.Fprintln(output, "if err:=validateDynamicNativeInput(ctx,data);err!=nil{return call,err}")
	hasBody := false
	for _, field := range operation.InputFields {
		hasBody = hasBody || field.Name == "Body"
	}
	if !hasBody {
		fmt.Fprintln(output, "if len(data.Binaries)!=0||len(data.Sources)!=0{return call,dynamicFailure(\"operation has no native request body\")}")
	}
	if operation.HasInput {
		fmt.Fprintln(output, "if value.Kind()!=types.ValueObject&&value.Kind()!=types.ValueNull{return call,dynamicFailure(\"operation input must be an object\")}")
		for _, field := range operation.InputFields {
			fmt.Fprintf(output, "if value,present:=value.Get(%q);present{", strings.ToLower(field.Name))
			fmt.Fprintf(output, "fieldData,_,err:=core.LinkDataAt(data,%q);if err!=nil{return call,err};_=fieldData;_=value;", "/"+strings.ToLower(field.Name))
			if field.Union {
				emitDynamicUnionBody(output, operation, field)
			} else {
				if !field.Binary && !field.Multipart && !field.Incremental {
					fmt.Fprintln(output, "if len(fieldData.Binaries)!=0||len(fieldData.Sources)!=0{return call,dynamicFailure(\"input section cannot contain native sources\")}")
				}
				emitDynamicDecode(output, field.Type, field.Binary, field.Multipart, field.Incremental, field.Stream, "value", "fieldData")
				fmt.Fprint(output, "if err!=nil{return call,err};call.input."+field.Name+"=")
				linkAssign(output, field, "decoded")
			}
			fmt.Fprint(output, "}")
			if field.Required {
				fmt.Fprint(output, "else{return call,dynamicFailure(\"required input section is missing\")}")
			}
			fmt.Fprintln(output)
		}
	}
	fmt.Fprintln(output, "return call,nil}")
}

func emitDynamicDecode(output *bytes.Buffer, model string, binary, multipart, incremental bool, stream *requestStreamPlan, value, data string) {
	switch {
	case binary:
		fmt.Fprintf(output, "if len(%s.Sources)!=0||len(%s.Binaries)>1{return call,dynamicFailure(\"binary body requires one complete binary source\")};for path:=range %s.Binaries{if path!=\"\"{return call,dynamicFailure(\"binary body source must replace the complete value\")}};", data, data, data)
		fmt.Fprintf(output, "decoded,present:=%s.Binaries[\"\"];if !present{decoded,err=dynamicBinary(%s)};", data, value)
	case incremental:
		fmt.Fprintf(output, "if len(%s.Sources)>1{return call,dynamicFailure(\"item body requires one complete item source\")};for path:=range %s.Sources{if path!=\"\"{return call,dynamicFailure(\"item source must replace the complete body value\")}};", data, data)
		fmt.Fprintf(output, "factory,present:=%s.Sources[\"\"];if !present{factory,err=core.LinkArrayFactory(%s);if err!=nil{return call,err}};decoded:=%s;", data, data, linkTargetFactoryExpression(stream, "factory"))

	case multipart:
		fmt.Fprintf(output, "if len(%s.Sources)!=0{return call,dynamicFailure(\"complete multipart body cannot contain item sources\")};", data)
		fmt.Fprintf(output, "decoded,err:=models.DecodeMedia%s(ctx,types.MultipartData{Value:%s,Slots:%s.Binaries});", model, value, data)
	default:
		fmt.Fprintf(output, "decoded,err:=models.Decode%s(ctx,%s);", model, value)
	}
}

func emitDynamicUnionBody(output *bytes.Buffer, operation operationPlan, field inputFieldPlan) {
	fmt.Fprintln(output, "typeValue,hasType:=value.Get(\"contentType\");contentType,isString:=typeValue.String();payload,hasValue:=value.Get(\"value\");if !hasType||!isString||!hasValue{return call,dynamicFailure(\"media body requires contentType and value\")};_=payload;payloadData,_,err:=core.LinkDataAt(fieldData,\"/value\");if err!=nil{return call,err};_=payloadData;selected,best:=0,0")
	fmt.Fprintln(output, "for path:=range fieldData.Binaries{if path!=\"/value\"&&!strings.HasPrefix(path,\"/value/\"){return call,dynamicFailure(\"native media value must belong to value\")}};for path:=range fieldData.Sources{if path!=\"/value\"{return call,dynamicFailure(\"item source must replace the media value\")}}")
	for _, body := range operation.RequestBodies {
		guard := body.Stream != nil && body.Stream.Complete && body.Stream.ItemValidation != ""
		if guard {
			if body.Incremental {
				fmt.Fprintln(output, "if _,factory:=payloadData.Sources[\"\"];factory{")
			} else {
				fmt.Fprintln(output, "if _,factory:=payloadData.Sources[\"\"];!factory{")
			}
		}
		fmt.Fprintf(output, "if score:=core.ResponseScore(200,contentType,\"200\",%q);score>best{selected,best=%d,score}\n", body.MediaType, body.Index)
		if guard {
			fmt.Fprintln(output, "}")
		}
	}
	fmt.Fprintln(output, "switch selected{")
	for _, body := range operation.RequestBodies {
		fmt.Fprintf(output, "case %d:", body.Index)
		if body.Open {
			fmt.Fprintf(output, "if binary,present:=payloadData.Binaries[\"\"];present{call.input.%s=", field.Name)
			linkAssign(output, field, fmt.Sprintf("models.New%s%sBinaryBody(contentType,binary)", operation.Name, body.Name))
			fmt.Fprintln(output, ";break}")
		}
		if !body.Binary && !body.Multipart && !body.Incremental {
			fmt.Fprintln(output, "if len(payloadData.Binaries)!=0||len(payloadData.Sources)!=0{return call,dynamicFailure(\"declared media value cannot contain native sources\")}")
		}
		emitDynamicDecode(output, body.Model, body.Binary, body.Multipart, body.Incremental, body.Stream, "payload", "payloadData")
		fmt.Fprint(output, "if err!=nil{return call,err};call.input."+field.Name+"=")
		argument := "decoded"
		if body.Wildcard {
			argument = "contentType," + argument
		}
		linkAssign(output, field, fmt.Sprintf("models.New%s%sBody(%s).WithContentType(contentType)", operation.Name, body.Name, argument))
		fmt.Fprintln(output)
	}
	fmt.Fprintln(output, "default:return call,dynamicFailure(\"request media type is not declared\")}")
}

func emitDynamicTerminals(output *bytes.Buffer, operation operationPlan) {
	fmt.Fprintf(output, "func(call %sCall)dynamicRaw(ctx context.Context)(*DynamicRawResponse,error){raw,err:=call.Raw(ctx);if raw==nil{return nil,err};result:=&DynamicRawResponse{Status:raw.Status,ContentType:raw.ContentType,Headers:raw.Headers.Clone(),Data:raw.Data,Request:raw.Request,OriginalResponse:raw.OriginalResponse,Body:http.NoBody,bytes:raw.Bytes(),sourceOperation:%q};", operation.Name, operation.Route)
	if operation.HasSequential {
		fmt.Fprint(output, "result.Body=raw.Body;")
	}
	if operation.LinkSource {
		fmt.Fprint(output, "result.source=raw.link;")
	}
	fmt.Fprintln(output, "return result,err}")
	if operation.HasTypedStream {
		emitDynamicStream(output, operation)
	}
	if !operation.CanDo {
		return
	}
	fmt.Fprintf(output, "func(call %sCall)dynamicDo(ctx context.Context)(value types.Value,failure error){var output %s;result,execution,err:=call.execute(ctx,&output);if err!=nil{return types.Value{},err};defer func(){if err:=execution.Close();err!=nil&&failure==nil{failure=execution.Failure(types.ResponseDecodeFailed,err)}}();value=result.value;switch result.branch{\n", operation.Name, operation.OutputType)
	for _, response := range operation.Responses {
		if !response.Success {
			continue
		}
		fmt.Fprintf(output, "case %d:\n", response.Index)
		switch {
		case response.Kind == "none":
			fmt.Fprintln(output, "return types.Value{},nil")
		case response.Kind == "binary":
			fmt.Fprintln(output, "return types.ValueFrom(result.body)")
		case response.Open:
			fmt.Fprintln(output, "if result.binary{return types.ValueFrom(result.body)}")
		case response.Buffered && response.Framing == "multipart":
			fmt.Fprintln(output, "value,err=dynamicMultipartValue(result.multipart);if err!=nil{return types.Value{},err}")
		}
		if response.DataModel != "" {
			fmt.Fprintln(output, "if data,present:=value.Get(\"data\");present{value=data}")
		}
		if response.Kind != "none" && response.Kind != "binary" {
			fmt.Fprintln(output, "return value,nil")
		}
	}
	fmt.Fprintln(output, "};return types.Value{},execution.Failure(types.ResponseDecodeFailed,dynamicFailure(\"response has no declared success contract\"))}")
}

func emitDynamicStream(output *bytes.Buffer, operation operationPlan) {
	fmt.Fprintf(output, "func(call %sCall)dynamicStream(ctx context.Context)*types.Stream[types.Value]{info:=types.OperationInfo{OperationID:%q,Route:%q,Method:%q,Path:%q};return types.NewStream(ctx,func(running context.Context)(types.Source[types.Value],*http.Response,error){result,execution,err:=call.execute(running,nil,true);if err!=nil{return nil,nil,err};switch result.branch{\n", operation.Name, operation.OperationID, operation.Route, operation.Method, operation.Path)
	for _, response := range operation.Responses {
		if !response.TypedStream {
			continue
		}
		fmt.Fprintf(output, "case %d:source,err:=completeSourceFor%s%s(execution,true);if err!=nil{execution.Close();return nil,result.OriginalResponse,err};execution.TransferStream();mapped:=types.MapSource(execution.Context,source,func(ctx context.Context,data types.MultipartData)(types.Value,error){flags:=make(map[string]bool,len(data.Slots));for path:=range data.Slots{flags[path]=true};if _,err:=schema.ValidateMedia__%s__(schema.ResponseContext(ctx),data.Value,\"\",flags);err!=nil{return types.Value{},err};return dynamicMultipartValue(data)});return mapped,result.OriginalResponse,nil\n", response.Index, operation.Name, response.Name, response.ItemValidation)
	}
	fmt.Fprintln(output, "};execution.Close();return nil,result.OriginalResponse,execution.Failure(types.ResponseDecodeFailed,dynamicFailure(\"response has no declared typed stream\"))},func(cause error,response *http.Response)error{return core.StreamFailure(ctx,info,cause,response)})}")
}
