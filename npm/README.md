# openapi-sdkgen

Generate application SDK source from OpenAPI 3.0, 3.1, and 3.2 documents.
The current release includes the `typescript` target.

```sh
pnpm dlx openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The npm launcher requires Node.js 22 or newer. This requirement applies only to
the npm launcher; generated SDK source has its own runtime compatibility
contract.

The npm package is a small JavaScript launcher. On the first run of a package
version, it downloads only the matching macOS, Linux, or Windows executable from
that exact version's GitHub Release, verifies it against the release SHA-256
checksums, and stores the verified executable in the user cache. Later runs reuse
the verified cache entry, so they do not require network access. Go is not
required by consumers.

Set `OPENAPI_SDKGEN_CACHE_DIR` to override the cache root, for example in an
isolated CI environment. The launcher never follows a moving `latest` release;
the npm package version and GitHub Release tag must match exactly.

For command reference and generated SDK usage, see the
[project documentation](https://jinyongp.github.io/openapi-sdkgen/).
