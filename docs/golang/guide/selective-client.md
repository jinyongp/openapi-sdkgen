# Select operations

Use the [Todo document](/examples/golang-todo.json) and [module setup](./getting-started.md).
The snippet runs in a function with `ctx context.Context` and an `error` return.

Static references preserve concrete operation types:

```go
ref := operations.GetTodo()
prepared, err := selective.Prepare(ref)
if err != nil {
    return err
}
client, err := selective.NewClient(selective.ClientOptions{BaseURL: "https://api.example.com"}, prepared)
if err != nil {
    return err
}
operation, err := selective.Bind(client, ref)
if err != nil {
    return err
}
todo, err := operation.Call(api.GetTodoInput{Path: api.GetTodoPath{ID: "todo-1"}}).Do(ctx)
```

Import `operations` and `selective` under your generated root. The linker removes unreachable
code, so Go needs no JavaScript bundler or tree-shaking configuration.
Imported package initialization can remain even when unused methods disappear.
Generation selection can also avoid importing unused runtime dependencies.
However, the compiler still checks imported packages. Use generation selection
(`--operation getTodo` or `--route 'GET /todos/{id}'`) to reduce generated source and
compile work. Link dependencies remain internal. Named clients in TOML have independent
entry packages. Computed `LookupOperation`/`LookupRoute` selection initializes its registry
lazily and returns a dynamic operation with `types.Value` or `types.NativeInput` inputs.
No fixed binary-size or build-speed improvement is promised.

Generation selection reduces emitted source, but the input document still needs to be
parsed and analyzed. Generation memory therefore need not fall in proportion to the
number of selected operations. For large specifications, measure generation and SDK
compilation separately, using the same input and selection for each comparison.

## Generation selection {#generation}

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api \
  --operation getTodo
```

The root exposes only selected public APIs. Required Link targets remain usable internally.
Unknown or hidden selections fail; selection does not skip parsing the original input.
See [named clients](./named-clients.md) for multiple fixed selections.
