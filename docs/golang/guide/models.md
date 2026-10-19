# Models and validation

The examples continue [First Go SDK](./getting-started.md). Import `types` from your generated root
(`example.com/consumer/internal/api/types`); `api` and `client` refer to that generated SDK.

<span id="typed-calls-and-presence"></span>


Required members use their generated type. Optional members use `types.Optional[T]`:
`types.Some(value)` is present, the zero value is absent. A nullable value uses
`types.Nullable[T]`; null and absence remain distinct. `types.Value` is an immutable JSON
wire tree, not a container for files or live streams. Input validation happens before HTTP;
response decoding preserves allowed unknown response properties. Standalone model codecs
and server output encoding still enforce their contracts.

```go
input := api.CreateTodoInput{Body: api.CreateTodoBody{Title: "Ship the SDK"}}
call := client.Operations.CreateTodo(input)
call = call.WithOptions(types.RequestOptions{RequestID: types.Some("example")})
```

See [types](../reference/types.md) for codecs, unions, enums and component projections.
