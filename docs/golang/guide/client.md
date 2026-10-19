# API calls and errors

Import your generated root as `api`, then construct a client and call an operation.
The snippets below run inside a function with a `ctx context.Context` and an `error` return.

```go
client, err := api.NewClient(api.ClientOptions{BaseURL: "https://api.example.com"})
if err != nil {
	return err
}
call := client.Operations.CreateTodo(api.CreateTodoInput{
	Body: api.CreateTodoBody{Title: "Ship the SDK"},
})
todo, err := call.Do(ctx)
if err != nil {
	return err
}
fmt.Println(todo.ID, todo.Title)
```

`Operations` preserves the operation API; `Routes` provides method/path entry points.
Resource methods bind path values, for example `client.Todos.ByID("todo-1").Get().Do(ctx)`.
Constructing a client, selector or call does not send HTTP. Every `Do`, `Raw` or stream
execution uses its own context. Calls can execute again when their input sources can reopen.



## Raw responses and Links

`Raw` retains status, headers and the full decoded value. Declared variant accessors such
as `As201JSON()` return a typed value and a boolean. Use the original raw response to follow
a Link; the generated Link resolves inputs and validates the target before a later execution.

```go
raw, err := call.Raw(ctx)
if err != nil {
	return err
}
fmt.Println(raw.Status, raw.Headers.Get("Content-Type"))
todo, err := client.Links.CreateTodo.GetTodo.Follow(raw).Do(ctx)
if err != nil {
	return err
}
fmt.Println(todo.ID)
```

Call `Raw` or `Do` once for a desired exchange: each method starts a separate execution.
For untyped sequential `Raw`, close its owned `Body` after reading. `Stream` closes its
response/source lease when exhausted, canceled, or explicitly closed; close it when stopping early.

## Handle errors

A missing import path is a generation error. A missing dependency is resolved in your own
module; generation does not install it. For incremental output, keep the managed manifest
with the directory. Inspect `types.APIError` using `errors.As` and stable error codes; error
text omits private request/response data:

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```
