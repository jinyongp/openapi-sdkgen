# Webhook receiver


Mount the generated `http.Handler` on your application's router.
[Download the complete Go Todo example](/downloads/golang-todo-app.zip) for the contract
and a runnable webhook delivery. Follow its README to run it.

## Connect the handler {#handler}

Generate the [Todo contract](/examples/golang-todo.json) with `--with server` and import
`server/webhooks` from the generated SDK. Inside a function, mount the handler on
an existing `http.ServeMux` named `mux`:

```go
handler, err := webhooks.NewCreatedPostHandler("/hooks/created", webhooks.Options{},
	func(ctx context.Context, input webhooks.CreatedPostInput) (webhooks.CreatedPostResponse, error) {
		fmt.Println(input.Body.ID)
		return webhooks.NewCreatedPost204NoContentResponse(), nil
	})
if err != nil {
	return err
}
mux.Handle("/hooks/created", handler)
```

For a JSON request body `{"id":"todo-1","title":"Ship the SDK"}`, the handler prints `todo-1` and replies with `204 No Content` and an empty body. Invalid input is rejected before this log runs.

The SDK validates input before invoking the handler and encodes its response.
Your application owns the listener, public route and authentication policy.

This contract allows anonymous webhook requests. Supply `Options.Authenticate`
for secured contracts. See [Server API](../reference/server-api.md) for request
bounds, callbacks and error handling.
