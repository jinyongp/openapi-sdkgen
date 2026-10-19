# Webhook receiver

Save the [Todo contract](/examples/golang-todo.json) as `openapi.json` and complete the
[module/dependency setup](../guide/getting-started.md). Enable server source:

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --with server
```

This complete `main.go` mounts the generated receiver on a local test server, sends a request,
and verifies status 204. It also demonstrates outbound calls, Links, NDJSON and static selection.
Use your application's listener and public route in production; callback URL registration belongs to the host.

```go
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	api "example.com/consumer/internal/api"
	"example.com/consumer/internal/api/operations"
	"example.com/consumer/internal/api/selective"
	"example.com/consumer/internal/api/server/webhooks"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/events":
			writer.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintln(writer, `{"id":"todo-1","title":"Ship the SDK"}`)
		case request.Method == "POST":
			writer.WriteHeader(201)
			fmt.Fprint(writer, `{"id":"todo-1","title":"Ship the SDK"}`)
		default:
			fmt.Fprint(writer, `{"id":"todo-1","title":"Ship the SDK"}`)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
	if err != nil {
		return err
	}
	raw, err := client.Operations.CreateTodo(api.CreateTodoInput{Body: api.CreateTodoBody{Title: "Ship the SDK"}}).Raw(ctx)
	if err != nil {
		return err
	}
	fmt.Println("created:", raw.Status)
	todo, err := client.Links.CreateTodo.GetTodo.Follow(raw).Do(ctx)
	if err != nil {
		return err
	}
	fmt.Println("linked:", todo.ID, todo.Title)
	todo, err = client.Todos.ByID("todo-1").Get().Do(ctx)
	if err != nil {
		return err
	}
	fmt.Println("resource:", todo.ID)
	stream := client.Operations.WatchTodos().Stream(ctx)
	defer stream.Close()
	for stream.Next() {
		fmt.Println("event:", stream.Item().Title)
	}
	if err := stream.Err(); err != nil {
		return err
	}
	ref := operations.GetTodo()
	prepared, err := selective.Prepare(ref)
	if err != nil {
		return err
	}
	selected, err := selective.NewClient(selective.ClientOptions{BaseURL: server.URL}, prepared)
	if err != nil {
		return err
	}
	operation, err := selective.Bind(selected, ref)
	if err != nil {
		return err
	}
	todo, err = operation.Call(api.GetTodoInput{Path: api.GetTodoPath{ID: "todo-1"}}).Do(ctx)
	if err != nil {
		return err
	}
	fmt.Println("selected:", todo.ID)
	handler, err := webhooks.NewCreatedPostHandler("/hooks/created", webhooks.Options{}, func(_ context.Context, input webhooks.CreatedPostInput) (webhooks.CreatedPostResponse, error) {
		fmt.Println("webhook:", input.Body.ID)
		return webhooks.NewCreatedPost204NoContentResponse(), nil
	})
	if err != nil {
		return err
	}
	hooks := httptest.NewServer(handler)
	defer hooks.Close()
	request, err := http.NewRequestWithContext(ctx, "POST", hooks.URL+"/hooks/created", strings.NewReader(`{"id":"todo-1","title":"Ship the SDK"}`))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := hooks.Client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return err
	}
	if response.StatusCode != 204 {
		return fmt.Errorf("webhook status: %d", response.StatusCode)
	}
	return nil
}
```

```sh
go run .
```

```text
created: 201
linked: todo-1 Ship the SDK
resource: todo-1
event: Ship the SDK
selected: todo-1
webhook: todo-1
```

The example is anonymous. Required schemes need `Options.Authenticate`; missing credentials return 401.
See [server API](../reference/server-api.md) for complete-body/frame bounds, duplex and failure handling.
