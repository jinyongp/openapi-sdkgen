# Choose a selection method

Select APIs during generation to reduce generated files. Choose an import method
to control how your application loads them.

| Goal | Use |
| --- | --- |
| Generate only the APIs your application uses | [API selection](./selective-client.md#generation) |
| Give pages or features separate API sets | [Named clients](./named-clients.md) |
| Choose APIs while the application runs | [`loadOperations`](./selective-client.md#prepare) |

## Generation and deployment

Generation-time selection includes the chosen APIs and their required Link
dependencies. Selecting every API does not reduce generated files.

Static imports let the bundler remove unused code. If you generate a full SDK
and import only a few operations, the application bundle can still be small.
Selecting APIs during generation also reduces the source that the compiler handles.

Dynamic lookup loads operation modules on demand. Native ESM deployment must
include the complete compiled lookup tree for the generated selection. A small
initial download does not mean a small total deployment.

Compare your application's initial downloads, deployed files, and build time.
Use [static imports](./selective-client.md#bundlers) when your bundler
needs to discover the dependencies, and follow the
[deployment guide](./selective-client.md#deployment) when serving native ESM.

## Published measurements

[Compatibility results](../reference/compatibility.md) publishes generation
counts and times extracted from GitHub Actions, with links to the measured run
and input documents. These results do not measure API server response time.
