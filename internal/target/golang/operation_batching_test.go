package golang

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedWideOperationBatches(t *testing.T) {
	parameters := make([]any, 0, 33)
	responses := make(map[string]any)
	var fields, assertions strings.Builder
	for index := range 33 {
		name := fmt.Sprintf("q%02d", index)
		parameters = append(parameters, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string"}})
		responses[fmt.Sprint(200+index)] = map[string]any{"description": "Done"}
		fmt.Fprintf(&fields, "Q%02d:types.Some(%q),", index, name)
		fmt.Fprintf(&assertions, "case %d:_,ok=output.As%dNoContent()\n", 200+index, 200+index)
	}
	input := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Wide", "version": "1"}, "paths": map[string]any{"/wide": map[string]any{"get": map[string]any{"operationId": "wide", "parameters": parameters, "responses": responses}}}}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, artifact := range artifacts {
		if !strings.HasPrefix(artifact.Path, "internal/operations/") || !strings.HasSuffix(artifact.Path, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), artifact.Path, artifact.Data, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			array, ok := literal.Type.(*ast.ArrayType)
			if !ok {
				return true
			}
			selector, ok := array.Elt.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "ParameterBinding" && selector.Sel.Name != "ResponseChoice" {
				return true
			}
			if len(literal.Elts) > 16 {
				t.Errorf("unbounded descriptor temporary: %s has %d elements", selector.Sel.Name, len(literal.Elts))
			}
			counts[selector.Sel.Name] += len(literal.Elts)
			return true
		})
	}
	if counts["ParameterBinding"] != 33 || counts["ResponseChoice"] != 33 {
		t.Fatalf("missing operation descriptors: %v", counts)
	}
	consumer := strings.NewReplacer("FIELDS", fields.String(), "ASSERTIONS", assertions.String()).Replace(wideOperationConsumer)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}

const wideOperationConsumer = `package consumer_test
import("context";"fmt";"net/http";"net/http/httptest";"strconv";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestWideOperation(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if len(r.URL.Query())!=33{t.Error("missing query parameters",r.URL)}
  for index:=range 33{name:=fmt.Sprintf("q%02d",index);if r.URL.Query().Get(name)!=name{t.Error("wrong query value",name,r.URL)}}
  status,err:=strconv.Atoi(r.Header.Get("X-Status"));if err!=nil{t.Error(err);status=500};w.WriteHeader(status)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 call:=client.Operations.Wide(sdk.WideInput{Query:types.Some(sdk.WideQuery{FIELDS})})
 for status:=200;status<233;status++{
  output,err:=call.WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Status":{strconv.Itoa(status)}}}).Do(context.Background());if err!=nil{t.Fatal(status,err)}
  ok:=false;switch status{ASSERTIONS};if !ok{t.Fatal("wrong selected branch across batch boundary",status)}
 }
}
`

func TestGeneratedOperationBatchingContracts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Batching","version":"1"},"paths":{"/":{"get":{"operationId":"health","responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/internal/core/batching_test.go": operationBatchingConsumer})
}

const operationBatchingConsumer = `package core
import("context";"errors";"net/http";"reflect";"testing";"example.test/consumer/api/types")

func TestParameterBatchOrderAndSnapshots(t *testing.T){
 section,err:=types.ValueFrom(map[string]any{"first":"one","custom":"two","last":"three"});if err!=nil{t.Fatal(err)}
 data:=RequestData{Path:"/resource"};calls:=0
 codec:=types.MediaCodec{EncodeParameter:func(_ context.Context,_ types.Value,metadata types.MediaContext)(string,error){
  calls++;if metadata.Request.URL.String()!="https://example.test/resource?first=one"||metadata.Request.Body!=http.NoBody||!reflect.DeepEqual(data.HeaderNames,[]string{"X-Missing"}){t.Fatal("snapshot order",metadata.Request,data.HeaderNames)}
  metadata.Request.Header.Set("X-Mutated","snapshot-only");return "encoded",nil
 }}
 client,err:=NewClient(ClientOptions{BaseURL:"https://example.test",Codecs:map[string]types.MediaCodec{"application/x-batch":codec}});if err!=nil{t.Fatal(err)}
 execution,err:=client.Begin(context.Background(),types.OperationInfo{Method:"GET"},types.RequestOptions{});if err!=nil{t.Fatal(err)};defer execution.Close()
 first:=[]ParameterBinding{{Section:section,Parameter:Parameter{Name:"first",Location:"query",Style:"form"}},{Section:section,Parameter:Parameter{Name:"X-Missing",Location:"header"}}}
 second:=[]ParameterBinding{{Section:section,Parameter:Parameter{Name:"custom",Location:"query",ContentType:"application/x-batch"}},{Section:section,Parameter:Parameter{Name:"last",Location:"header"}}}
 if err:=execution.AddParameters(&data,first);err!=nil{t.Fatal(err)}
 if err:=execution.AddParameters(&data,second);err!=nil{t.Fatal(err)}
 if calls!=1||data.Query!="first=one&custom=encoded"||data.Headers.Get("last")!="three"||data.Headers.Get("X-Mutated")!=""||!reflect.DeepEqual(data.HeaderNames,[]string{"X-Missing","last"}){t.Fatal("prepared values",calls,data)}
}
func TestParameterBatchStopsAtFirstErrorAndCancellation(t *testing.T){
 sentinel:=errors.New("encode failed");section,_:=types.ValueFrom(map[string]any{"fail":"value","later":"value"})
 for _,cancelled:=range []bool{false,true}{
  ctx,cancel:=context.WithCancel(context.Background());if cancelled{cancel()};calls:=0
  execution:=&Execution{Context:ctx};data:=RequestData{}
  err:=execution.AddParameters(&data,[]ParameterBinding{{Section:section,Parameter:Parameter{Name:"missing",Location:"header"}},{Section:section,Parameter:Parameter{Name:"fail",Location:"header",ContentType:"application/x-batch",Encoder:func(context.Context,types.Value,string)([]byte,error){calls++;return nil,sentinel}}},{Section:section,Parameter:Parameter{Name:"later",Location:"header"}}})
  cancel();expected:=sentinel;expectedCalls:=1;if cancelled{expected=context.Canceled;expectedCalls=0}
  if !errors.Is(err,expected)||calls!=expectedCalls||!reflect.DeepEqual(data.HeaderNames,[]string{"missing","fail"})||data.Headers!=nil{t.Fatal("first failure",cancelled,err,calls,data)}
 }
 // Missing values do not acquire an extra cancellation check.
 ctx,cancel:=context.WithCancel(context.Background());cancel();data:=RequestData{}
 if err:=(&Execution{Context:ctx}).AddParameters(&data,[]ParameterBinding{{Parameter:Parameter{Name:"absent",Location:"header"}}});err!=nil||!reflect.DeepEqual(data.HeaderNames,[]string{"absent"}){t.Fatal("missing semantics",err,data)}
}
func TestResponseSelectionAcrossBatches(t *testing.T){
 choices:=make([]ResponseChoice,33)
 for index:=range choices{choices[index]=ResponseChoice{Index:100+index,Status:"404",MediaType:"application/json",Kind:"none"}}
 choices[15]=ResponseChoice{Index:715,Status:"default",Kind:"binary",BinaryFallback:true}
 choices[16]=ResponseChoice{Index:716,Status:"2XX",MediaType:"application/*",Kind:"text"}
 choices[17]=ResponseChoice{Index:717,Status:"200",MediaType:"application/json",Kind:"json"}
 choices[32]=ResponseChoice{Index:732,Status:"200",MediaType:"application/json",Kind:"blocked",BinaryFallback:true}
 for _,test:=range []struct{status int;media string;branch int;kind string;binary bool}{
  {200,"Application/JSON; charset=utf-8",717,"json",false},
  {201,"application/problem+json",716,"text",false},
  {500,"",715,"binary",true},
  {404,"application/json",100,"none",false},
 }{
  selection:=ResponseSelection{Kind:"auto"}
  for start:=0;start<len(choices);start+=16{selection.Choose(test.status,test.media,choices[start:min(start+16,len(choices))])}
  if selection.Branch!=test.branch||selection.Kind!=test.kind||selection.BinaryFallback!=test.binary{t.Fatal(test,selection)}
 }
 selection:=ResponseSelection{Kind:"auto"};selection.Choose(200,"application/json",nil)
 if selection.Branch!=0||selection.Score!=0||selection.Kind!="auto"||selection.BinaryFallback{t.Fatal("empty selection",selection)}
 selection.Choose(200,"application/problem+json",[]ResponseChoice{{Index:8,Status:"200",MediaType:"application/*+json",Kind:"json"},{Index:9,Status:"200",MediaType:"application/*",Kind:"binary"}})
 if selection.Branch!=8||selection.Kind!="json"{t.Fatal("suffix specificity",selection)}
}
`
