# Webhooks and callbacks

Generate with `--with server`. Your application owns the listener and callback URL registration.

Import `server/webhooks` or `server/callbacks` from the generated root. Mount a typed handler
at a host-selected path; your application owns the listener and callback URL registration:

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

Required authentication uses `Options.Authenticate`. Parameters are decoded before this
hook; it receives original security requirements/candidates, five parameter groups, context,
and an HTTP metadata snapshot with `http.NoBody`. A nil rejection continues; a rejection
returns its status/headers/source. Anonymous alternatives skip the hook. Missing authentication
returns401; a hook error returns500. `CloneBody(ctx)` returns an independent reader for
bounded complete input; the caller closes the clone. Incremental input cannot be cloned.

Complete input defaults to8 MiB, with413 on overflow. Incremental frames default to1 MiB,
with400 on overflow; the aggregate stream has no complete-body cap. Override the limits with
`MaxBodyBytes` and `MaxStreamFrameBytes`. HTTP/1 duplex requires a writer that supports
`EnableFullDuplex`, and streaming needs flush support. HTTP/2 uses native multiplexing.
A failure after headers aborts the response and reaches `OnError`; it cannot replace the
already sent status. Keep input sources alive until the response finishes when echoing input.
The SDK closes the lease after sending.

Import the generated `server/runtime` package as `runtime` and the generated `types` package for the normalized-schema example below.

`server/runtime.CompileSchema` is the separate generic path for normalized schemas and a
definition map. It accepts `types`, `reference`, `constValue` and other normalized keywords,
not raw JSON Schema `type` or `$ref`. It never fetches URLs or executes schema extensions:

```go
schemaJSON := []byte(`{"types":["object"],"properties":{"count":{"types":["integer"]}},"required":["count"]}`)
normalized, err := types.ParseValue(schemaJSON)
if err != nil {
	return err
}
contract, err := runtime.CompileSchema(normalized, nil)
if err != nil {
	return err
}
value, err := contract.Decode(ctx, []byte(`{"count":2}`))
if err != nil {
	return err
}
return contract.Validate(ctx, value)
```

See [server API](../reference/server-api.md) for limits and host integration.
