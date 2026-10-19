# Security and transport

Use `ClientOptions.SecurityProvider` to return credentials for the declared scheme identities.
Direct credentials and scoped server selection use generated types; alternatives and scheme
metadata are available to your provider. `HTTPClient` and `Transport` are alternatives.
A custom transport must declare whether response bytes are encoded or already decoded with
original headers. The SDK decodes response compression and keeps original headers. Do not
silently decompress while retaining an encoded capability declaration.

Request timeout and cancellation use the execution context. HTTP/1 connection reuse needs a
consumed or closed response. Host transports and hooks must support concurrent invocations.
Go accepts valid HTTP token methods and does not apply browser Fetch's forbidden-header or
method rules. A separate CONNECT tunnel API is not provided.

See [client options](../reference/client-api.md) and [stream lifetimes](../reference/streaming.md).
