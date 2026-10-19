# Go SSE consumer

Install Go 1.27+, Bash and the `openapi-sdkgen` CLI. Run from this directory:

```sh
bash ./setup.sh
go run .
```

The application starts a temporary local SSE server, calls it through the generated
SDK, prints the received text, then closes the stream and server. No API key or
external service is required. Start with `main.go` to connect your own server;
keep the context deadline, stream cleanup and error handling.
