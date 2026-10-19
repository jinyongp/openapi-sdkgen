# AI streaming examples

## Contract

Replace the fixed SSE consumers with runnable applications:
generated SDK client → local HTTP server → external AI library → model provider.
The server publishes the model's text as SSE and owns provider credentials.
Documentation keeps the key call and recognizable output; downloads contain
the complete setup. Changes use patches, with project formatters for formatting.

TypeScript uses `ai` 7.0.143 and `@ai-sdk/openai` 4.0.96; Go uses the official
`openai-go/v3` v3.82.0 in a separate server module. Both call Chat Completions.
The generated Go client retains its standard-library-only consumer module.
Model IDs are required configuration rather than a hardcoded model recommendation.
The examples bind loopback and do not implement production application authentication.

## Review coverage

| Area | Result |
| --- | --- |
| Server dependency and model configuration | Pinned manifests and locks; startup rejects missing key/model; base URL configurable |
| Client/server boundary | Real HTTP requests; removed custom Fetch and temporary fixed SSE server |
| Streaming and failure handling | Incremental output, bounded prompt input, 502 before output, interrupted response after output |
| Lifetime | Client Ctrl+C and two-minute timeout; server propagates disconnection upstream |
| Download contents | Allowlist includes the TS server and Go server module/checksums; integration fragment removed |
| Documentation | Both SDKs, EN/KO pages and example indexes; snippets and example output match across locales |

The first review caught Go accepting a valid JSON object followed by garbage;
require EOF and cover it in the shared server integration checks. It also caught
default AI SDK error logging; use a generic diagnostic instead of dumping provider errors.
Recheck changed servers through their downloaded archives after these fixes.

## Verification

Passed after the review fixes: `devtools run build:docs`, `devtools run verify:docs`,
`devtools run fmt-check:dev`, and `git diff --check`. Verification covers all eight
built example archives, source inventory/determinism, documentation TypeScript,
built routes/links and locale checks. Automated checks use a test-only provider
HTTP fixture and the real installed provider libraries. The fixture asserts the
Chat Completions path, streaming flag, model, prompt and authorization header.
It verifies successful output, invalid input, provider authentication failure,
interrupted provider output, generated client failure reporting, and cancellation
propagation. It is not included in public downloads.

No paid model calls are made. Live inference with a user's key/model is unverified.
Provider installation needs network/cache access; Go consumer generation and
execution are separately checked with an empty module cache and network disabled.

## Suggested next examples

1. Background report job: implemented in the follow-up
   [report application work](report-jobs-example-review.md), covering submit → poll
   progress → download a real generated file and a separate cancel path.
2. File conversion: implemented in [the file conversion applications](file-conversion-example-review.md),
   covering upload → JSON-to-NDJSON conversion → saved download and readable result.
3. Signed webhook delivery: implemented in [the signed webhook applications](signed-webhook-example-review.md),
   covering separate signed delivery, generated receiver validation and deduplicated effects.

Implement each as a complete small application for both clients, with observable
results and a short documentation overview. Keep application-level job state,
signatures and deduplication outside the generator's responsibilities.
