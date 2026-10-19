# Troubleshooting

## Generation fails

Read the diagnostic's code, location and hint. Check the declared OpenAPI version,
resolve missing references and supply required schema extensions. See
[OpenAPI support](../reference/capabilities.md) and [remote inputs](./remote-inputs.md).

## Generated output conflicts

Retain the managed manifest and regenerate with the same input/options. Keep hand-written
files outside generator-owned paths. See [generation modes](./generate.md).

## Imports or type checking fail

Use your application's module resolution. Direct Node ESM execution uses explicit `.js`
entry paths after compilation. Generation does not require TypeScript; optional checks
must use a [supported compiler](../reference/types.md#compiler-support).

## A request or stream fails

Inspect the typed error and raw response described in [client API](../reference/client-api.md).
Check [authentication and transport](./transport.md), propagate cancellation, and close
streams when a consumer stops before the end.
