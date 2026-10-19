package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedResourceCalls(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Resource calls","version":"1"},"paths":{"/todos/{todoID}":{"get":{"operationId":"getTodo","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"query":{"type":"string"}}}}}}},"parameters":[{"name":"todoID","in":"path","required":true,"schema":{"type":"string","minLength":2}},{"name":"q","in":"query","required":false,"schema":{"type":"string"}}]}},"/todos":{"post":{"operationId":"createTodo","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"query":{"type":"string"}}}}}}},"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["title"],"properties":{"title":{"type":"string"}}}}}}}},"/customers/{customerID}/widgets/{widgetID}":{"get":{"operationId":"getWidget","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"query":{"type":"string"}}}}}}},"parameters":[{"name":"customerID","in":"path","required":true,"schema":{"type":"string"}},{"name":"widgetID","in":"path","required":true,"schema":{"type":"integer","minimum":1}},{"name":"tag","in":"query","required":false,"schema":{"type":"string"}}]}},"/todos/{todoID}/notes":{"post":{"operationId":"createNote","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"query":{"type":"string"}}}}}}},"parameters":[{"name":"todoID","in":"path","required":true,"schema":{"type":"string","minLength":2}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["text"],"properties":{"text":{"type":"string"}}}}}}}},"/mixed/{key}.json":{"get":{"operationId":"mixedRoute","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"query":{"type":"string"}}}}}}},"parameters":[{"name":"key","in":"path","required":true,"schema":{"type":"string"}}]}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	requireResourceWarnings(t, diagnostics, 1)
	testGeneratedModule(t, plan, map[string]string{"resource_test.go": resourceConsumer})
}

const resourceConsumer = `package consumer_test
import("context";"encoding/json";"errors";"net/http";"net/http/httptest";"reflect";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestResourcesReuseTypedCallsAndBoundPaths(t *testing.T){
 var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);if r.Method=="POST"{var body map[string]any;if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Error(err)}};w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(map[string]any{"path":r.URL.Path,"query":r.URL.RawQuery})}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 id:="todo/1";todo:=client.Todos.ByID(id);call:=todo.Get(sdk.GetTodoResourceInput{Query:types.Some(sdk.GetTodoQuery{Q:types.Some("filter")})});if requests.Load()!=0{t.Fatal("selectors or call construction performed HTTP")};if _,present:=reflect.TypeOf(sdk.GetTodoResourceInput{}).FieldByName("Path");present{t.Fatal("bound path leaked into resource input")}
 for range 2{result,err:=call.Do(context.Background());if err!=nil||result.Path!="/todos/todo/1"||result.Query!="q=filter"{t.Fatal("bound get",result,err)}}
 widget,err:=client.Customers.ByID("c-1").Widgets.ByID(3).Get(sdk.GetWidgetResourceInput{Query:types.Some(sdk.GetWidgetQuery{Tag:types.Some("same")})}).Do(context.Background());if err!=nil||widget.Path!="/customers/c-1/widgets/3"||widget.Query!="tag=same"{t.Fatal("nested bindings",widget,err)}
 created,err:=client.Todos.Create(sdk.CreateTodoInput{Body:sdk.CreateTodoBody{Title:"title"}}).Do(context.Background());if err!=nil||created.Path!="/todos"{t.Fatal("unbound canonical input",created,err)}
 note,err:=todo.Notes.Create(sdk.CreateNoteResourceInput{Body:sdk.CreateNoteBody{Text:"note"}}).Do(context.Background());if err!=nil||note.Path!="/todos/todo/1/notes"{t.Fatal("nested body",note,err)}
 exact,err:=client.Routes.GetTodosByID(sdk.GetTodoInput{Path:sdk.GetTodoPath{TodoID:"todo-2"}}).Do(context.Background());if err!=nil||exact.Path!="/todos/todo-2"{t.Fatal("typed route entry",exact,err)}
 before:=requests.Load();invalid:=client.Todos.ByID("x").Get();if requests.Load()!=before{t.Fatal("invalid selector performed HTTP")};_,err=invalid.Do(context.Background());var failure *sdk.APIError;if !errors.As(err,&failure)||failure.Code!=types.RequestEncodeFailed||requests.Load()!=before{t.Fatal("deferred operation validation",err,requests.Load())}
 _,err=todo.Get(sdk.GetTodoResourceInput{},sdk.GetTodoResourceInput{}).Do(context.Background());if err==nil||requests.Load()!=before{t.Fatal("resource input arity",err)}
}
`
