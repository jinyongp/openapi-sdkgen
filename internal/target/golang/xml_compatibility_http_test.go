package golang

import (
	"encoding/json"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLLegacyAndNodeTypeAcrossVersions(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			payload := func(modern bool) map[string]any {
				attribute := map[string]any{"attribute": true}
				wrapped := map[string]any{"name": "tags", "wrapped": true}
				if modern {
					attribute = map[string]any{"nodeType": "attribute"}
					wrapped = map[string]any{"name": "tags", "nodeType": "element"}
				}
				return map[string]any{"type": "object", "xml": map[string]any{"name": "payload"}, "required": []string{"id", "tags"}, "properties": map[string]any{
					"id":   map[string]any{"type": "integer", "xml": attribute},
					"tags": map[string]any{"type": "array", "xml": wrapped, "items": map[string]any{"type": "string", "xml": map[string]any{"name": "tag"}}},
				}}
			}
			operation := func(name string, modern bool) map[string]any {
				media := map[string]any{"application/xml": map[string]any{"schema": payload(modern)}}
				return map[string]any{"post": map[string]any{"operationId": name, "requestBody": map[string]any{"required": true, "content": media}, "responses": map[string]any{"200": map[string]any{"description": "Done", "content": media}}}}
			}
			paths := map[string]any{"/legacy": operation("saveLegacyXML", false)}
			if version == "3.2.0" {
				paths["/modern"] = operation("saveModernXML", true)
			}
			data, err := json.Marshal(map[string]any{"openapi": version, "info": map[string]any{"title": "XML compatibility", "version": "1"}, "paths": paths})
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
			files := map[string]string{"xml_compat_test.go": xmlCompatibilityConsumer}
			if version == "3.2.0" {
				files["xml_modern_test.go"] = xmlModernCompatibilityConsumer
			}
			testGeneratedModule(t, plan, files)
		})
	}
}

const xmlCompatibilityConsumer = `package consumer_test
import("context";"encoding/json";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func xmlCompatibilityClient(t *testing.T)*sdk.Client{
 t.Helper();server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){body,err:=io.ReadAll(request.Body);if err!=nil||string(body)!="<payload id=\"7\"><tags><tag>one</tag><tag>two</tag></tags></payload>"{t.Error("XML compatibility request",string(body),err)};writer.Header().Set("Content-Type","application/xml");io.WriteString(writer,"<payload id=\"8\"><tags><tag>red</tag><tag>blue</tag></tags></payload>")}));t.Cleanup(server.Close);client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};return client
}
func checkXMLCompatibilityResult(t *testing.T,result any){t.Helper();data,err:=json.Marshal(result);if err!=nil{t.Fatal(err)};value,err:=types.ParseValue(data);if err!=nil{t.Fatal(err)};expected,_:=types.ParseValue([]byte("{\"id\":8,\"tags\":[\"red\",\"blue\"]}"));if !value.Equal(expected){t.Fatal("XML compatibility response",string(data))}}
func TestLegacyXMLHTTP(t *testing.T){client:=xmlCompatibilityClient(t);value,_:=types.ParseValue([]byte("{\"id\":7,\"tags\":[\"one\",\"two\"]}"));body,err:=models.DecodeSaveLegacyXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)};result,err:=client.Operations.SaveLegacyXML(sdk.SaveLegacyXMLInput{Body:body}).Do(context.Background());if err!=nil{t.Fatal(err)};checkXMLCompatibilityResult(t,result)}
`

const xmlModernCompatibilityConsumer = `package consumer_test
import("context";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestNodeTypeXMLHTTP(t *testing.T){client:=xmlCompatibilityClient(t);value,_:=types.ParseValue([]byte("{\"id\":7,\"tags\":[\"one\",\"two\"]}"));body,err:=models.DecodeSaveModernXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)};result,err:=client.Operations.SaveModernXML(sdk.SaveModernXMLInput{Body:body}).Do(context.Background());if err!=nil{t.Fatal(err)};checkXMLCompatibilityResult(t,result)}
`
