# Webhooks and callbacks



Generate inbound handlers with `--with server` and mount them on your application's router.
[Download the Go Todo example](/downloads/golang-todo-app.zip) for complete runnable code.

## Register a handler

Import the generated `server/webhooks` package and attach a handler to an existing `http.ServeMux`:

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

## Authentication and request bounds

Supply `Options.Authenticate` for secured contracts. Without it, secured requests
return `401`. Operations allowing anonymous access skip the hook.

Complete bodies default to an 8 MiB limit; incremental frames default to 1 MiB.
Override them with `MaxBodyBytes` and `MaxStreamFrameBytes`.

See [Server API](../reference/server-api.md) for callbacks, duplex behavior,
error handling and dynamic schema validation.
