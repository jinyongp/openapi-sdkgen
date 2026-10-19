package golang

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func normalizedTestSchema(node *schemaplan.Node) map[string]any {
	result := make(map[string]any)
	for _, field := range node.Fields {
		switch value := field.Value.(type) {
		case schemaplan.Literal:
			result[field.Name] = value.Data
		case schemaplan.Child:
			result[field.Name] = normalizedTestSchema(value.Node)
		case schemaplan.Children:
			children := make([]any, len(value.Nodes))
			for index, child := range value.Nodes {
				children[index] = normalizedTestSchema(child)
			}
			result[field.Name] = children
		case schemaplan.Properties:
			children := make(map[string]any)
			for _, property := range value.Entries {
				children[property.Name] = map[string]any{"property": property.Name, "schema": normalizedTestSchema(property.Schema)}
			}
			result[field.Name] = children
		case schemaplan.SchemaMap:
			children := make(map[string]any)
			for _, property := range value.Entries {
				children[property.Name] = normalizedTestSchema(property.Schema)
			}
			result[field.Name] = children
		case schemaplan.DynamicReference:
			result[field.Name] = map[string]any{"anchor": value.Anchor, "fallback": normalizedTestSchema(value.Fallback)}
		}
	}
	return result
}

func TestGeneratedGenericInboundOfficialSchemaCases(t *testing.T) {
	for _, keyword := range []string{"unevaluatedProperties", "unevaluatedItems"} {
		t.Run(keyword, func(t *testing.T) {
			data, err := os.ReadFile("../../../test/typescript/fixtures/json-schema-" + keyword + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var groups []schemaSuiteGroup
			if err := json.Unmarshal(data, &groups); err != nil {
				t.Fatal(err)
			}
			components := make(map[string]any)
			for index, group := range groups {
				if object, ok := group.Schema.(map[string]any); ok {
					if _, present := object["$id"]; !present {
						object["$id"] = fmt.Sprintf("https://schemas.example.test/%s/%d", keyword, index)
					}
				}
				components[fmt.Sprintf("Case%d", index)] = group.Schema
			}
			encoded, err := json.Marshal(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Generic official cases", "version": "1"}, "paths": map[string]any{}, "components": map[string]any{"schemas": components}})
			if err != nil {
				t.Fatal(err)
			}
			document, err := compiler.Compile(encoded)
			if err != nil {
				t.Fatal(err)
			}
			definitions := make(map[string]any)
			for name, source := range document.Schemas {
				node, err := schemaplan.Lower(source.Value, schemaplan.Options{Projection: schemaplan.Input, ReferenceName: schemaplan.ComponentReferenceName})
				if err != nil {
					t.Fatal(err)
				}
				definitions[name] = normalizedTestSchema(node)
			}
			encoded, err = json.Marshal(definitions)
			if err != nil {
				t.Fatal(err)
			}
			var consumer strings.Builder
			fmt.Fprintf(&consumer, "package consumer_test\nimport(\"context\";\"testing\";\"example.test/consumer/api/types\";\"example.test/consumer/api/server/runtime\")\nfunc TestOfficialGeneric(t *testing.T){raw,err:=types.ParseValue([]byte(%q));if err!=nil{t.Fatal(err)};definitions:=make(map[string]types.Value);for _,key:=range raw.Keys(){definitions[key],_=raw.Get(key)}\n", string(encoded))
			count := 0
			for index, group := range groups {
				fmt.Fprintf(&consumer, "{contract,err:=runtime.CompileSchema(definitions[%q],definitions);if err!=nil{t.Fatal(err)}\n", fmt.Sprintf("Case%d", index))
				for _, test := range group.Tests {
					fmt.Fprintf(&consumer, "t.Run(%q,func(t *testing.T){value,err:=types.ParseValue([]byte(%q));if err!=nil{t.Fatal(err)};err=contract.Validate(context.Background(),value);if (err==nil)!=%t{t.Fatalf(\"valid=%%t error=%%v\",%t,err)}})\n", group.Description+"/"+test.Description, string(test.Data), test.Valid, test.Valid)
					count++
				}
				consumer.WriteString("}\n")
			}
			consumer.WriteString("}\n")
			addons, _ := generator.NewAddonRegistry(generator.AddonServer)
			options, _ := addons.Resolve([]string{"server"})
			options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
			plan, findings, err := (Generator{}).Prepare(document, options)
			if err != nil || diagnostic.HasErrors(findings) {
				t.Fatal(err, findings)
			}
			testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer.String()})
			t.Logf("%d official generic cases across %d groups", count, len(groups))
		})
	}
}

func TestGeneratedGenericInboundRuntime(t *testing.T) {
	document, err := compiler.Compile([]byte(healthDocument))
	if err != nil {
		t.Fatal(err)
	}
	addons, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := addons.Resolve([]string{"server"})
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	plan, findings, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(findings) {
		t.Fatal(err, findings)
	}
	root := testGeneratedModule(t, plan, map[string]string{"consumer_test.go": genericInboundConsumer + genericInboundCompositionConsumer, "api/server/runtime/limits_test.go": genericInboundLimitsConsumer})
	checkInboundImportGraph(t, root, "runtime", false)
}

const genericInboundConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"strings";"sync";"sync/atomic";"testing";"example.test/consumer/api/types";"example.test/consumer/api/server/runtime")
func value(t *testing.T,text string)types.Value{t.Helper();value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};return value}
func TestGenericDecodeRespondAndDefinitions(t *testing.T){definitions:=map[string]types.Value{"Count":value(t,"{\"types\":[\"integer\"],\"minimum\":1}")};contract,err:=runtime.CompileSchema(value(t,"{\"types\":[\"object\"],\"properties\":{\"count\":{\"property\":\"count\",\"schema\":{\"reference\":\"Count\"}}},\"required\":[\"count\"],\"additionalProperties\":false}"),definitions);if err!=nil{t.Fatal(err)};definitions["Count"]=value(t,"{\"boolean\":false}");decoded,err:=contract.Decode(context.Background(),[]byte("\ufeff{\"count\":2}"));if err!=nil{t.Fatal(err)};if err:=contract.Validate(context.Background(),decoded);err!=nil{t.Fatal("caller mutation changed definitions",err)};handler,err:=runtime.NewHandler("/",runtime.Definition{Method:"POST"},runtime.Options{},func(execution *runtime.Execution,_ runtime.ParameterResult)(runtime.Response,error){input,err:=contract.DecodeInbound(execution,execution.Request.Header.Get("Content-Type"),"json");if err!=nil{return runtime.Response{},err};return contract.Respond(execution,201,"application/json","json",input)});if err!=nil{t.Fatal(err)};for _,test:=range []struct{body string;status int}{ {"{\"count\":2}",201},{"{\"count\":0}",400},{"{\"count\":\"2\"}",400},{"{\"count\":2,\"extra\":1}",400} }{request:=httptest.NewRequest("POST","/",strings.NewReader(test.body));request.Header.Set("Content-Type","application/json");recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);if recorder.Code!=test.status{t.Fatal(test,recorder.Code,recorder.Body.String())};if test.status==201&&recorder.Body.String()!=test.body{t.Fatal(recorder.Body.String())}}}
func TestGenericFormatsContentCancellationAndConcurrentReuse(t *testing.T){for _,test:=range []struct{format,good,bad string}{ {"email","a@example.test","invalid"},{"date","2024-02-29","2023-02-29"},{"ipv6","::1","1.2.3.4"},{"uuid","12345678-1234-1234-1234-123456789abc","bad"} }{root,_:=types.ValueFrom(map[string]any{"types":[]string{"string"},"format":test.format,"formatAssertion":true});contract,err:=runtime.CompileSchema(root,nil);if err!=nil{t.Fatal(err)};good,_:=types.ValueFrom(test.good);bad,_:=types.ValueFrom(test.bad);if contract.Validate(context.Background(),good)!=nil||contract.Validate(context.Background(),bad)==nil{t.Fatal(test.format)}};contract,err:=runtime.CompileSchema(value(t,"{\"types\":[\"string\"],\"pattern\":\"^[a-z]+$\"}"),nil);if err!=nil{t.Fatal(err)};good,_:=types.ValueFrom("hello");var group sync.WaitGroup;for i:=0;i<16;i++{group.Add(1);go func(){defer group.Done();for j:=0;j<20;j++{if err:=contract.Validate(context.Background(),good);err!=nil{t.Error(err)}}}()};group.Wait();ctx,cancel:=context.WithCancel(context.Background());cancel();if err:=contract.Validate(ctx,good);!errors.Is(err,context.Canceled){t.Fatal(err)};content,err:=runtime.CompileSchema(value(t,"{\"types\":[\"string\"],\"contentEncoding\":\"base64\",\"contentMediaType\":\"application/json\",\"contentSchema\":{\"types\":[\"integer\"],\"minimum\":2}}"),nil);if err!=nil{t.Fatal(err)};for _,test:=range []struct{wire string;valid bool}{ {"Mg==",true},{"MQ==",false},{"@@",false} }{wire,_:=types.ValueFrom(test.wire);if (content.Validate(context.Background(),wire)==nil)!=test.valid{t.Fatal(test)}}}
func TestGenericNeverFetchesReferences(t *testing.T){var calls atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls.Add(1);io.WriteString(w,"{} ")}));defer server.Close();root,_:=types.ValueFrom(map[string]any{"reference":server.URL+"/schema"});if _,err:=runtime.CompileSchema(root,nil);err==nil{t.Fatal("missing external definition accepted")};if calls.Load()!=0{t.Fatal("generic compile fetched a URL")};root=value(t,"{\"types\":[\"integer\"],\"program\":{\"source\":\"throw new Error('must not run')\"},\"x-untrusted\":\"unexecuted\"}");contract,err:=runtime.CompileSchema(root,nil);if err!=nil{t.Fatal(err)};number,_:=types.ValueFrom(2);if err:=contract.Validate(context.Background(),number);err!=nil{t.Fatal(err)}}
func TestResponseStatusPrecedence(t *testing.T){declared:=[]string{"200","2XX","default"};for _,test:=range []struct{status int;variant string;valid bool}{ {200,"200",true},{200,"2XX",false},{200,"default",false},{201,"2XX",true},{404,"default",true},{199,"default",false} }{if runtime.ResponseStatus(test.status,test.variant,declared)!=test.valid{t.Fatal(test)}}}
func TestGenericXMLAndXMLContent(t *testing.T){root:=value(t,"{\"types\":[\"object\"],\"xml\":{\"name\":\"record\",\"prefix\":\"r\",\"namespace\":\"urn:records\"},\"properties\":{\"id\":{\"schema\":{\"reference\":\"Count\",\"xml\":{\"attribute\":true}}},\"body\":{\"schema\":{\"types\":[\"string\"],\"xml\":{\"nodeType\":\"cdata\"}}},\"tags\":{\"schema\":{\"types\":[\"array\"],\"xml\":{\"wrapped\":true},\"items\":{\"types\":[\"integer\"],\"xml\":{\"name\":\"tag\"}}}}},\"required\":[\"id\",\"body\",\"tags\"],\"additionalProperties\":false}");contract,err:=runtime.CompileSchema(root,map[string]types.Value{"Count":value(t,"{\"types\":[\"integer\"],\"minimum\":1}")});if err!=nil{t.Fatal(err)};expected:=value(t,"{\"id\":7,\"body\":\"hello <xml>\",\"tags\":[1,2]}");xml:=[]byte("<r:record xmlns:r=\"urn:records\" id=\"7\"><![CDATA[hello <xml>]]><tags><tag>1</tag><tag>2</tag></tags></r:record>");decoded,err:=contract.DecodeXML(context.Background(),xml);if err!=nil||!decoded.Equal(expected){t.Fatal(decoded,err)};encoded,err:=contract.EncodeXML(context.Background(),expected);if err!=nil{t.Fatal(err)};again,err:=contract.DecodeXML(context.Background(),encoded);if err!=nil||!again.Equal(expected){t.Fatal(string(encoded),again,err)};handler,err:=runtime.NewHandler("/",runtime.Definition{Method:"POST"},runtime.Options{},func(execution *runtime.Execution,_ runtime.ParameterResult)(runtime.Response,error){input,err:=contract.DecodeInbound(execution,"application/xml","xml");if err!=nil{return runtime.Response{},err};return contract.Respond(execution,200,"application/xml","xml",input)});if err!=nil{t.Fatal(err)};request:=httptest.NewRequest("POST","/",strings.NewReader(string(xml)));request.Header.Set("Content-Type","application/xml");recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);if recorder.Code!=200{t.Fatal(recorder.Code,recorder.Body.String())};output,err:=contract.DecodeXML(context.Background(),recorder.Body.Bytes());if err!=nil||!output.Equal(expected){t.Fatal(output,err)};content,err:=runtime.CompileSchema(value(t,"{\"types\":[\"string\"],\"contentMediaType\":\"application/xml\",\"contentSchema\":{\"types\":[\"object\"],\"properties\":{\"id\":{\"types\":[\"integer\"],\"minimum\":1}},\"required\":[\"id\"]}}"),nil);if err!=nil{t.Fatal(err)};for _,test:=range []struct{text string;valid bool}{ {"<root><id>2</id></root>",true},{"<root><id>0</id></root>",false},{"<root>",false} }{item,_:=types.ValueFrom(test.text);if (content.Validate(context.Background(),item)==nil)!=test.valid{t.Fatal(test)}}}

func TestGenericRejectsMalformedContractsAndXMLReferenceOverride(t *testing.T){for _,text:=range []string{"{\"type\":\"integer\"}","{\"$ref\":\"https://example.test/schema\"}","{\"types\":[\"unknown\"]}","{\"minimum\":\"2\"}","{\"maxItems\":-1}","{\"multipleOf\":0}","{\"required\":[1]}","{\"pattern\":\"[\"}","{\"patternProperties\":{\"[\":{}}}","{\"dependentRequired\":{\"x\":false}}"}{if _,err:=runtime.CompileSchema(value(t,text),nil);err==nil{t.Fatal("invalid normalized contract accepted",text)}};contract,err:=runtime.CompileSchema(value(t,"{\"reference\":\"Base\",\"xml\":{\"name\":\"local\"}}"),map[string]types.Value{"Base":value(t,"{\"types\":[\"integer\"],\"xml\":{\"name\":\"base\"}}")});if err!=nil{t.Fatal(err)};number,_:=types.ValueFrom(1);encoded,err:=contract.EncodeXML(context.Background(),number);if err!=nil||string(encoded)!="<local>1</local>"{t.Fatal(string(encoded),err)}}

`

const genericInboundCompositionConsumer = `
func TestGenericDynamicScopeAndConditionalEvaluation(t *testing.T){
 definitions:=map[string]types.Value{
 "Node":value(t,"{\"dynamicAnchor\":\"node\",\"types\":[\"object\"],\"properties\":{\"data\":{\"types\":[\"integer\"]},\"children\":{\"types\":[\"array\"],\"items\":{\"dynamicReference\":{\"anchor\":\"node\",\"fallback\":{\"reference\":\"Node\"}}}}}}"),
 "Strict":value(t,"{\"dynamicAnchor\":\"node\",\"reference\":\"Node\",\"unevaluatedProperties\":false}"),
 }
 contract,err:=runtime.CompileSchema(definitions["Strict"],definitions);if err!=nil{t.Fatal(err)}
 for _,test:=range []struct{text string;valid bool}{
 {"{\"data\":1,\"children\":[{\"data\":2}]}",true},
 {"{\"data\":1,\"children\":[{\"data\":2,\"unknown\":true}]}",false},
 {"{\"data\":1,\"children\":[{\"data\":\"bad\"}]}",false},
 }{if err:=contract.Validate(context.Background(),value(t,test.text));(err==nil)!=test.valid{t.Fatal(test,err)}}
 root:=value(t,"{\"types\":[\"object\"],\"properties\":{\"kind\":{\"types\":[\"string\"]}},\"if\":{\"required\":[\"kind\"],\"properties\":{\"kind\":{\"constValue\":\"list\"}}},\"then\":{\"required\":[\"items\"],\"properties\":{\"items\":{\"types\":[\"array\"],\"contains\":{\"types\":[\"integer\"]},\"minContains\":2,\"maxContains\":2}}},\"else\":{\"properties\":{\"value\":{\"types\":[\"string\"]}}},\"unevaluatedProperties\":false}")
 contract,err=runtime.CompileSchema(root,nil);if err!=nil{t.Fatal(err)}
 for _,test:=range []struct{text string;valid bool}{
 {"{\"kind\":\"list\",\"items\":[1,\"a\",2]}",true},
 {"{\"kind\":\"list\",\"items\":[1,2,3]}",false},
 {"{\"kind\":\"other\",\"value\":\"yes\"}",true},
 {"{\"kind\":\"other\",\"items\":[1,2]}",false},
 }{if err:=contract.Validate(context.Background(),value(t,test.text));(err==nil)!=test.valid{t.Fatal(test,err)}}
}
`
