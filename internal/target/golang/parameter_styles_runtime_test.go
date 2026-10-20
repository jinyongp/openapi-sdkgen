package golang

import "testing"

func TestGeneratedParameterWireStyles(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/core/parameters_test.go": parameterStyleConsumer})
}

const parameterStyleConsumer = `package core
import("context";"testing";"example.test/consumer/api/types")
func TestParameterStyles(t *testing.T){
 cases:=[]struct{name,location,style,value,want string;explode,reserved bool;media string}{
  {"label","path","label","[\"a b\",\"c\"]","/x/.a%20b,c",false,false,""},
  {"label-explode","path","label","[\"a b\",\"c\"]","/x/.a%20b.c",true,false,""},
  {"matrix","path","matrix","{\"a\":1,\"b\":null}","/x/;a=1;b=null",true,false,""},
  {"null","path","simple","null","/x/null",false,false,""},
  {"query-array","query","form","[\"a\",\"b\"]","p=a,b",false,false,""},
  {"query-pipe-object","query","pipeDelimited","{\"a\":1,\"b\":2}","p=a%7C1%7Cb%7C2",false,false,""},
  {"query-pipe-explode","query","pipeDelimited","{\"a\":1,\"b\":2}","p=a%3D1%7Cb%3D2",true,false,""},
  {"query-space-explode","query","spaceDelimited","{\"a\":1,\"b\":2}","p=a%3D1%20b%3D2",true,false,""},
  {"reserved","query","form","\"%2F:/?#&+[],\"","p=%2F:/?%23%26%2B%5B%5D,",false,true,""},
  {"header-object","header","simple","{\"a\":1,\"b\":null}","a=1,b=null",true,false,""},
  {"cookie-array","cookie","form","[\"a b\",null]","p=a%20b%2Cnull",false,false,""},
  {"cookie-object","cookie","form","{\"a\":1,\"b\":null}","p=a%2C1%2Cb%2Cnull",false,false,""},
  {"cookie-null","cookie","form","null","p=null",true,false,""},
  {"cookie-array-null","cookie","form","[null]","p=",true,false,""},
  {"raw-cookie","cookie","cookie","{\"a\":\"x:y\"}","a=x:y",true,false,""},
  {"json-content","query","form","{\"a\":1}","p=%7B%22a%22%3A1%7D",true,false,"application/json"},
  {"text-null-content","query","form","null","p=null",true,false,"text/plain"},
  {"form-content","query","form","{\"a\":[\"x y\",\"z\"],\"b\":\"~*\"}","p=a%3Dx%2By%26a%3Dz%26b%3D%257E*",true,false,"application/x-www-form-urlencoded"},
  {"querystring-form","querystring","","{\"a\":[\"x y\",\"z\"],\"b\":\"~*\"}","a=x%20y&a=z&b=~*",false,false,"application/x-www-form-urlencoded"},
 }
 client,err:=NewClient(ClientOptions{BaseURL:"http://example.test"});if err!=nil{t.Fatal(err)}
 for _,test:=range cases{t.Run(test.name,func(t *testing.T){execution,err:=client.Begin(context.Background(),types.OperationInfo{Method:"GET",Path:"/x/{p}"},types.RequestOptions{});if err!=nil{t.Fatal(err)};defer execution.Close()
  value,err:=types.ParseValue([]byte(test.value));if err!=nil{t.Fatal(err)}
  data:=RequestData{Path:"/x/{p}"};err=execution.AddParameter(&data,value,Parameter{Name:"p",Location:test.location,Style:test.style,Explode:test.explode,AllowReserved:test.reserved,ContentType:test.media})
  if err!=nil{t.Fatal(err)}
  actual:=data.Query;switch test.location{case "path":actual=data.Path;case "header":actual=data.Headers.Get("p");case "cookie":actual=data.Headers.Get("Cookie")}
  if actual!=test.want{t.Fatalf("wire: %q, expected %q",actual,test.want)}
 })}
 execution,err:=client.Begin(context.Background(),types.OperationInfo{Method:"GET"},types.RequestOptions{});if err!=nil{t.Fatal(err)};defer execution.Close()
 value,_:=types.ValueFrom("a/b");data:=RequestData{}
 if err:=execution.AddParameter(&data,value,Parameter{Name:"p/x",Location:"query",Style:"form",AllowReserved:true});err!=nil{t.Fatal(err)}
 if data.Query!="p%2Fx=a/b"{t.Fatal("allowReserved leaked into query name",data.Query)}
}
`
