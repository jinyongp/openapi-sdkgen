package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedInboundSlowConsumerCancellation(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound backpressure","version":"1"},"paths":{},"webhooks":{"events":{"post":{"responses":{"200":{"description":"Events","content":{"application/x-ndjson":{"itemSchema":{"type":"string"}}}}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "ITEMMODEL", entry.Operation.Responses[0].ItemModel, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"errors";"net/http";"net/http/httptest";"strings";"sync";"sync/atomic";"testing";"time";"example.test/consumer/api/models";"example.test/consumer/api/server/webhooks")
type pacedSource struct{ctx context.Context;item models.ITEMMODEL;next,closes atomic.Int32;first chan struct{};closed chan error;once sync.Once}
func(source *pacedSource)Next()bool{if source.ctx.Err()!=nil{return false};count:=source.next.Add(1);source.once.Do(func(){close(source.first)});return count<=32}
func(source *pacedSource)Item()models.ITEMMODEL{return source.item}
func(source *pacedSource)Err()error{if source.next.Load()>32{return errors.New("output was drained before consumption")};return source.ctx.Err()}
func(source *pacedSource)Close()error{source.closes.Add(1);source.closed<-source.ctx.Err();return nil}
func TestSlowConsumerAndCancellation(t *testing.T){for _,http2:=range []bool{false,true}{t.Run(map[bool]string{false:"http1",true:"http2"}[http2],func(t *testing.T){source:=&pacedSource{item:models.ITEMMODEL(strings.Repeat("x",512*1024)),first:make(chan struct{}),closed:make(chan error,2)};handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(ctx context.Context,_ webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){source.ctx=ctx;return webhooks.NewPUBLICVARIANTResponse(source),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewUnstartedServer(handler);server.EnableHTTP2=http2;if http2{server.StartTLS()}else{server.Start()};defer server.Close();ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();request,_:=http.NewRequestWithContext(ctx,"POST",server.URL,nil);response,err:=server.Client().Do(request);if err!=nil{t.Fatal(err)};defer response.Body.Close();select{case <-source.first:case <-time.After(3*time.Second):t.Fatal("source did not start")};time.Sleep(50*time.Millisecond);if count:=source.next.Load();count>=32{t.Fatal("slow consumer did not stop source pulls",count)};cancel();response.Body.Close();select{case err:=<-source.closed:if !errors.Is(err,context.Canceled){t.Fatal("source closed before request cancellation",err)};case <-time.After(3*time.Second):t.Fatal("disconnected consumer left source open")};if source.closes.Load()!=1{t.Fatal("source closed multiple times",source.closes.Load())}})}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
