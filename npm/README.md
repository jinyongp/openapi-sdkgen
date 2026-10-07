# openapi-sdkgen

Generate application SDK source from OpenAPI 3.0, 3.1, and 3.2 documents.
The current release includes the `typescript` target.

Use Node.js 22+ and your own `openapi.yaml`. For a complete runnable example,
follow [Getting started](https://jinyongp.dev/openapi-sdkgen/guide/getting-started.html).

```sh
pnpm dlx openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The npm launcher requires Node.js 22 or newer. This requirement applies only to
the npm launcher; generated SDK source has its own runtime compatibility
contract.

The package downloads and verifies the matching native executable from the
same-version GitHub Release on first execution. The first run needs network
access; later runs reuse the verified cache and can work offline. Go is not
required.

Set `OPENAPI_SDKGEN_CACHE_DIR` to override the cache root, for example in an
isolated CI environment. The executable version always matches the installed
npm package version.

For command reference and generated SDK usage, see the
[project documentation](https://jinyongp.dev/openapi-sdkgen/).
