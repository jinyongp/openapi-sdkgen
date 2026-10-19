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

With the Todo example server, the output is:

```text
todo-1 Ship the SDK
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

With the Todo example server, the output is:

```text
201 application/json
todo-1
```

Call `Raw` or `Do` once for a desired exchange: each method starts a separate execution.
For untyped sequential `Raw`, close its owned `Body` after reading. `Stream` closes its
response/source lease when exhausted, canceled, or explicitly closed; close it when stopping early.

## Handle errors

Import the generated `types` package and use `errors.As` to recognize SDK errors.
Use their stable codes for application error handling.

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```

For a call canceled through its context, this prints `REQUEST_ABORTED`. If `err` is `nil` or does not contain an SDK `APIError`, the branch prints nothing.
