package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestGeneratedTypedInboundResponseVariants(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Variants","version":"1"},"paths":{},"webhooks":{"received":{"post":{"responses":{"200":{"description":"Exact","content":{"application/json":{"schema":{"type":"string"}}}},"2XX":{"description":"Range","content":{"application/x-ndjson":{"schema":{"type":"array","items":{"type":"integer"}}}}},"default":{"description":"Fallback","content":{"text/plain":{"schema":{"type":"string"}}}}}}}}}`
	document, err := compiler.Compile([]byte(source))
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
	entries, findings := prepareInboundOperations(document, inboundScopes(document, nil, nil), names, new(schemaPlanner))
	if diagnostic.HasErrors(findings) || len(entries) != 1 {
		t.Fatal(findings)
	}
	entry := entries[0]
	replacements := []string{"PUBLIC", entry.PublicName}
	for _, response := range entry.Operation.Responses {
		key := "EXACT"
		if response.Status == "2XX" {
			key = "RANGE"
		} else if response.Status == "default" {
			key = "FALLBACK"
		}
		replacements = append(replacements, key+"VARIANT", response.Name, key+"MODEL", response.Model)
	}
	consumer := strings.NewReplacer(replacements...).Replace(`package consumer_test
import("context";"net/http/httptest";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/webhooks")
func TestResponseVariants(t *testing.T){for _,test:=range []struct{mode string;status,want int;body string}{ {"exact",200,200,"\"okay\""},{"range",201,201,"1\n2\n"},{"range",200,500,""},{"range",199,500,""},{"fallback",418,418,"okay"},{"fallback",201,500,""} }{handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(ctx context.Context,_ webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){text,_:=types.ValueFrom("okay");switch test.mode{case "exact":typed,err:=models.DecodeEXACTMODEL(ctx,text);if err!=nil{return webhooks.PUBLICResponse{},err};return webhooks.NewPUBLICEXACTVARIANTResponse(typed),nil;case "range":raw,_:=types.ParseValue([]byte("[1,2]"));typed,err:=models.DecodeRANGEMODEL(ctx,raw);if err!=nil{return webhooks.PUBLICResponse{},err};return webhooks.NewPUBLICRANGEVARIANTCompleteResponse(test.status,typed),nil;default:typed,err:=models.DecodeFALLBACKMODEL(ctx,text);if err!=nil{return webhooks.PUBLICResponse{},err};return webhooks.NewPUBLICFALLBACKVARIANTResponse(test.status,typed),nil}});if err!=nil{t.Fatal(err)};recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,httptest.NewRequest("POST","/",nil));if recorder.Code!=test.want||test.body!=""&&recorder.Body.String()!=test.body{t.Fatal(test,recorder.Code,recorder.Body.String())}}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
