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

func TestGeneratedInboundSSEDefaultContract(t *testing.T) {
	encoded, err := os.ReadFile("../../../test/fixtures/support-gaps/sse-event.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	paths := raw["paths"].(map[string]any)
	webhooks := make(map[string]any)
	for _, pair := range [][2]string{{"batch", "/publish-buffered"}, {"stream", "/publish-incremental"}, {"both", "/both"}} {
		operation := paths[pair[1]].(map[string]any)["post"].(map[string]any)
		webhooks[pair[0]] = map[string]any{"post": map[string]any{"requestBody": operation["requestBody"], "responses": map[string]any{"204": map[string]any{"description": "OK"}}}}
	}
	raw["paths"], raw["webhooks"] = map[string]any{}, webhooks
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
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
	names, _ := planNames(document)
	schemas := &schemaPlanner{reference: func(string, schemaplan.Projection) (string, error) { return "checkComponent", nil }}
	entries, findings := prepareInboundOperations(document, inboundScopes(document, nil, nil), names, schemas)
	if diagnostic.HasErrors(findings) {
		t.Fatal(findings)
	}
	var body strings.Builder
	body.WriteString(`package consumer_test
import("context";"net/http";"net/http/httptest";"strings";"testing";"example.test/consumer/api/models";"example.test/consumer/api/server/webhooks")
func TestSSEContracts(t *testing.T){handlers:=map[string]http.Handler{};seen:=map[string]string{}
`)
	for _, entry := range entries {
		field := entry.Operation.InputFields[0]
		fmt.Fprintf(&body, "{handler,err:=webhooks.New%sHandler(\"/\",webhooks.Options{},func(ctx context.Context,input webhooks.%sInput)(webhooks.%sResponse,error){\n", entry.PublicName, entry.PublicName, entry.PublicName)
		if field.Incremental {
			fmt.Fprintf(&body, "var count int;for input.Body.Next(){raw,err:=models.Encode%s(ctx,input.Body.Item());if err!=nil{return webhooks.%sResponse{},err};data,_:=raw.MarshalJSON();seen[%q]+=string(data);count++};if count>2{t.Fatal(count)}\n", field.Stream.ItemModel, entry.PublicName, entry.Definition.Name)
		} else {
			fmt.Fprintf(&body, "raw,err:=models.Encode%s(ctx,input.Body);if err!=nil{return webhooks.%sResponse{},err};for _,item:=range raw.Items(){data,_:=item.MarshalJSON();seen[%q]+=string(data)}\n", field.Type, entry.PublicName, entry.Definition.Name)
		}
		fmt.Fprintf(&body, "return webhooks.New%s%sResponse(),nil});if err!=nil{t.Fatal(err)};handlers[%q]=handler}\n", entry.PublicName, entry.Operation.Responses[0].Name, entry.Definition.Name)
	}
	body.WriteString(`wire:="event: message\ndata: plain\nid: evt1\nretry: 4\n\nevent: json\ndata: {\"a\":1}\nid: \nretry: 0\n\n";expected:="{\"data\":\"plain\",\"event\":\"message\",\"id\":\"evt1\",\"retry\":4}{\"data\":\"{\\\"a\\\":1}\",\"event\":\"json\",\"id\":\"\",\"retry\":0}";for name,handler:=range handlers{request:=httptest.NewRequest("POST","/",strings.NewReader(wire));request.Header.Set("Content-Type","text/event-stream");recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);if recorder.Code!=204||seen[name]!=expected{t.Fatal(name,recorder.Code,seen[name],expected,recorder.Body.String())};request=httptest.NewRequest("POST","/",strings.NewReader("data: \n\n"));request.Header.Set("Content-Type","text/event-stream");recorder=httptest.NewRecorder();handler.ServeHTTP(recorder,request);if recorder.Code!=400{t.Fatal("invalid inbound Event accepted",name,recorder.Code)}}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": body.String()})
}
