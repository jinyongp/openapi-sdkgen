# openapi-sdkgen

Generate application SDK source from OpenAPI 3.0, 3.1, and 3.2 documents.
Choose `typescript` or `golang`. The generated source belongs to your application;
no separate SDK runtime package is required.

The npm launcher requires Node.js 22+. Use your own `openapi.yaml`, or follow a
complete local example: [TypeScript](https://jinyongp.dev/openapi-sdkgen/typescript/guide/getting-started.html)
or [Go](https://jinyongp.dev/openapi-sdkgen/golang/guide/getting-started.html).

```sh
pnpm dlx openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

For Go, set the generated package's full import path:

```sh
pnpm dlx openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api
```

Match the import path to your Go module. SDK consumption requires Go 1.27+ and uses
only the standard library. No additional runtime modules are needed.

The npm launcher requires Node.js 22 or newer. This requirement applies only to
the npm launcher; generated SDK source has its own runtime compatibility
contract.

The package downloads and verifies the matching native executable from the
same-version GitHub Release on first execution. The first run needs network
access; later runs reuse the verified cache and can work offline. Go is not
required to generate source. Consuming a generated Go SDK requires Go 1.27+.

Set `OPENAPI_SDKGEN_CACHE_DIR` to override the cache root, for example in an
isolated CI environment. The executable version always matches the installed
npm package version.

For command reference and generated SDK usage, see the
[project documentation](https://jinyongp.dev/openapi-sdkgen/).
