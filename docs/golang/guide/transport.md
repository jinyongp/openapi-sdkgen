# Security and transport

Use `ClientOptions.SecurityProvider` to return credentials for the declared scheme identities.
Direct credentials and scoped server selection use generated types; alternatives and scheme
metadata are available to your provider. `HTTPClient` and `Transport` are alternatives.

## Control response compression

Your `http.Client` or `http.RoundTripper` owns compression negotiation and decoding.
The SDK adds no `Accept-Encoding` and does not change `http.Transport.DisableCompression`.
Go's default HTTP transport can negotiate and decode gzip itself. To control that default:

```go
transport := http.DefaultTransport.(*http.Transport).Clone()
transport.DisableCompression = true
client, err := api.NewClient(api.ClientOptions{
	BaseURL:    "https://api.example.com",
	HTTPClient: &http.Client{Transport: transport},
})
```

`NewClient` returns a configured client or an error; it sends no request. Later calls use a transport that does not automatically request or decode gzip.

Configure your own transport for Brotli, Zstandard or other encodings. Advertise an encoding
only when your HTTP layer can decode it. Return decoded bytes for JSON, XML and stream
validation, or supply a media codec that handles the returned representation. The SDK uses
the body and metadata your HTTP layer returns and closes the body when execution ends.

The SDK does not reconstruct headers removed by HTTP decoding. Required declared response
headers are checked against the headers actually returned. Binary and sequential `Raw`
expose the returned bytes; JSON and other structured `Raw` responses keep their normal media
decoding and schema validation. See [stream lifetimes](../reference/streaming.md).

## Cancellation and connection reuse

Request timeout and cancellation use the execution context. HTTP/1 connection reuse needs a
consumed or closed response. Host transports and hooks must support concurrent invocations.
Go accepts valid HTTP token methods and does not apply browser Fetch's forbidden-header or
method rules. A separate CONNECT tunnel API is not provided.

See [client options](../reference/client-api.md).
