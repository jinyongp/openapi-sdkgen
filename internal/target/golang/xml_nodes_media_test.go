package golang

import (
	"encoding/json"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

const xmlNameBoundaryConsumer = `package wire
import("context";"errors";"testing")
func TestUnicodeXMLNamesDoNotChangeContentOrCollide(t *testing.T){
 const source="<SdkgenXMLName0><💩 xmlns:🚀=\"urn:rocket\" 🚀:💩=\"SdkgenXMLName1 💩\">text 💩<!-- <💩> --><![CDATA[<🚀:💩>]]><🚀:💩/></💩></SdkgenXMLName0>"
 root,err:=ParseXML(context.Background(),source);if err!=nil{t.Fatal(err)}
 if root.Name.Local!="SdkgenXMLName0"||len(root.Children)!=1{t.Fatal("ASCII name collided with alias",root)};node:=root.Children[0]
 if node.Name.Local!="💩"||node.Text!="text 💩<🚀:💩>"||len(node.Attributes)!=1||node.Attributes[0].Name.Local!="💩"||node.Attributes[0].Name.Prefix!="🚀"||node.Attributes[0].Name.Namespace!="urn:rocket"||node.Attributes[0].Value!="SdkgenXMLName1 💩"||len(node.Children)!=1||node.Children[0].Name.Local!="💩"||node.Children[0].Name.Namespace!="urn:rocket"{t.Fatal("name fallback changed content/namespaces",node)}
}
func TestUnicodeXMLNameFallbackKeepsFailures(t *testing.T){
 for _,source:=range []string{"<💩></🚀>","<💩 a=\"1\" a=\"2\"/>","<💩 p:a=\"1\"/>","<💩 xmlns:p=\"urn:x\" xmlns:q=\"urn:x\" p:a=\"1\" q:a=\"2\"/>","<💩><1bad/></💩>","<💩/><second/>","<!DOCTYPE 💩><💩/>","<💩 xmlns:p=\"\"/>","<💩>&unknown;</💩>","<💩><\U000f0000/></💩>"}{if _,err:=ParseXML(context.Background(),source);err==nil{t.Fatal("invalid XML accepted",source)}}
 ctx,cancel:=context.WithCancel(context.Background());cancel();if _,err:=ParseXML(ctx,"<💩/>");!errors.Is(err,context.Canceled){t.Fatal("cancellation lost",err)}
}
`

func TestGeneratedXMLNodeTypesNamespacesAndNames(t *testing.T) {
	rich := map[string]any{"type": "object", "required": []string{"content", "id", "ignored", "items"}, "xml": map[string]any{"name": "root"}, "properties": map[string]any{
		"content": map[string]any{"type": "string", "xml": map[string]any{"nodeType": "cdata"}},
		"id":      map[string]any{"type": "integer", "xml": map[string]any{"nodeType": "attribute", "prefix": "a", "namespace": "urn:attribute"}},
		"ignored": map[string]any{"type": "string", "xml": map[string]any{"nodeType": "none"}},
		"items":   map[string]any{"type": "array", "xml": map[string]any{"name": "item", "prefix": "p", "namespace": "urn:items"}, "items": map[string]any{"type": "string"}},
	}}
	collision := map[string]any{"type": "object", "required": []string{"id"}, "xml": map[string]any{"name": "root", "prefix": "p", "namespace": "urn:root"}, "properties": map[string]any{"id": map[string]any{"type": "integer", "xml": map[string]any{"attribute": true, "prefix": "p", "namespace": "urn:attribute"}}}}
	unicode := map[string]any{"type": "string", "xml": map[string]any{"name": "💩"}}
	paths := make(map[string]any)
	for _, entry := range []struct {
		name   string
		schema any
	}{{"Rich", rich}, {"Collision", collision}, {"Unicode", unicode}} {
		responses := map[string]any{"204": map[string]any{"description": "Done"}}
		paths["/"+entry.name] = map[string]any{"post": map[string]any{"operationId": "send" + entry.name, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/xml": map[string]any{"schema": entry.schema}}}, "responses": responses}}
	}
	paths["/Unicode"].(map[string]any)["get"] = map[string]any{"operationId": "getUnicode", "responses": map[string]any{"200": map[string]any{"description": "Unicode name", "content": map[string]any{"application/xml": map[string]any{"schema": unicode}}}}}
	data, err := json.Marshal(map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "XML node contract", "version": "1"}, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"xml_nodes_test.go": xmlNodesHTTPConsumer, "api/internal/wire/xml_name_test.go": xmlNameBoundaryConsumer})
}

const xmlNodesHTTPConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestXMLNodesAndNamespaceOwnership(t *testing.T){
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);if request.Method=="GET"{writer.Header().Set("Content-Type","application/xml");io.WriteString(writer,"<💩>x</💩>");return};body,err:=io.ReadAll(request.Body);if err!=nil{t.Error(err)};switch request.URL.Path{
 case "/Rich":if string(body)!="<root xmlns:a=\"urn:attribute\" a:id=\"1\"><![CDATA[a]]]]><![CDATA[>b]]><p:item xmlns:p=\"urn:items\">one</p:item><p:item xmlns:p=\"urn:items\">two</p:item></root>"{t.Error("CDATA/attribute/none/unwrapped namespace",string(body))}
 case "/Unicode":if string(body)!="<💩>x</💩>"{t.Error("Unicode name",string(body))}
 case "/Collision":t.Error("conflicting namespace declarations reached HTTP",string(body))
 };writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,err:=types.ParseValue([]byte("{\"content\":\"a]]>b\",\"id\":1,\"ignored\":\"omit\",\"items\":[\"one\",\"two\"]}"));if err!=nil{t.Fatal(err)}
 rich,err:=models.DecodeSendRichBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendRich(sdk.SendRichInput{Body:rich}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendUnicode(sdk.SendUnicodeInput{Body:sdk.SendUnicodeBody("x")}).Do(context.Background());err!=nil{t.Fatal(err)}
 result,err:=client.Operations.GetUnicode().Do(context.Background());if err!=nil||string(result)!="x"{t.Fatal("Unicode XML decode",result,err)}
 count:=requests.Load();value,err=types.ParseValue([]byte("{\"id\":1}"));if err!=nil{t.Fatal(err)};collision,err:=models.DecodeSendCollisionBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 _,err=client.Operations.SendCollision(sdk.SendCollisionInput{Body:collision}).Do(context.Background());var failure *types.APIError;if !errors.As(err,&failure)||failure.Code!=types.RequestEncodeFailed||requests.Load()!=count{t.Fatal("namespace collision did not fail before HTTP",err,requests.Load(),count)}
}
`
