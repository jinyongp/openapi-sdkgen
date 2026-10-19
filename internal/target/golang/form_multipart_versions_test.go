package golang

import (
	"encoding/json"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedFormAndNullableMultipartAcrossVersions(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			file := map[string]any{"type": "string", "nullable": true, "format": "binary"}
			if version != "3.0.3" {
				file = map[string]any{"type": []string{"string", "null"}, "format": "binary"}
			}
			schema := map[string]any{"type": "object", "required": []string{"file", "label", "payload", "tags"}, "properties": map[string]any{
				"file": file, "label": map[string]any{"type": "string"},
				"payload": map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "integer"}}},
				"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}}
			paths := make(map[string]any)
			for _, entry := range []struct{ path, operation, media string }{{"/form", "sendFormValue", "application/x-www-form-urlencoded"}, {"/multipart", "sendMultipartValue", "multipart/form-data"}} {
				paths[entry.path] = map[string]any{"post": map[string]any{"operationId": entry.operation, "requestBody": map[string]any{"required": true, "content": map[string]any{entry.media: map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}}
			}
			data, err := json.Marshal(map[string]any{"openapi": version, "info": map[string]any{"title": "Form and multipart versions", "version": "1"}, "paths": paths})
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
			testGeneratedModule(t, plan, map[string]string{"form_multipart_versions_test.go": formMultipartVersionConsumer})
		})
	}
}

const formMultipartVersionConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestFormAndNullableMultipartWire(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){if request.URL.Path=="/form"{body,err:=io.ReadAll(request.Body);if err!=nil||string(body)!="file=null&label=hello&id=1&tags=a&tags=b"{t.Error("form wire",string(body),err)}}else{
 _,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);names:=[]string{};bodies:=[]string{};for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};body,err:=io.ReadAll(part);if err!=nil{t.Error(err)};names=append(names,part.FormName());bodies=append(bodies,string(body));contentType,filename:="","";if part.FormName()=="file"{contentType,filename="application/octet-stream","blob"};if part.FormName()=="payload"{contentType,filename="application/json","blob"};if part.Header.Get("Content-Type")!=contentType||part.FileName()!=filename{t.Error("nullable/structured multipart metadata",part.Header)}};if strings.Join(names,",")!="file,label,payload,tags,tags"||strings.Join(bodies,"|")!="null|hello|{\"id\":1}|a|b"{t.Error("multipart wire",names,bodies)}
 };writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};value,_:=types.ParseValue([]byte("{\"file\":null,\"label\":\"hello\",\"payload\":{\"id\":1},\"tags\":[\"a\",\"b\"]}"))
 form,err:=models.DecodeSendFormValueBody(context.Background(),value);if err!=nil{t.Fatal(err)};if _,err:=client.Operations.SendFormValue(sdk.SendFormValueInput{Body:form}).Do(context.Background());err!=nil{t.Fatal(err)}
 body,err:=models.DecodeSendMultipartValueBody(context.Background(),value);if err!=nil{t.Fatal(err)};if _,err:=client.Operations.SendMultipartValue(sdk.SendMultipartValueInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
