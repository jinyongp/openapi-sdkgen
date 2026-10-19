# Public documentation reading-flow review

Status: initial pass and follow-up complete. The follow-up below records the missing results and unclear request-stream example identified after the initial pass.

## Contract

The guides explain generation and key application calls. Complete runnable setup,
contracts, local servers, compiler configuration and executable checks live in
downloadable examples. API references retain detailed contracts. English and Korean
use identical code examples. Preserve previous links and generated SDK behavior.

Scope: all 114 current public pages, the root README and all seven example READMEs.
Historical redirect pages are checked through the existing built-route validator.
Internal architecture documents, measurement reports and implementation plans are
outside this reader-facing rewrite.

## Findings and resolution

- First-use pages required copying full contracts, mock HTTP implementations and
  compiler flags to obtain an uninformative count. Replace them with generation and
  application-call summaries, linking runnable Todo downloads.
- Streaming and webhook pages embedded whole programs and test setup. Move complete
  code into examples; retain key call/handler code and usage conditions.
- Adjacent guides referred to the removed mock/ESM tutorial. Repair those entry paths.
- Go guides duplicated raw responses, source ownership and dynamic server details
  from references. Keep practical usage in guides and lookup detail in references.
- TypeScript selection internals obscured the guide. Move container, enumeration,
  computed-membership and preparation-error detail to the client reference.
- Example setup prerequisites had drifted from their pinned packages. Document
  Node.js 24, pnpm 12 and Bash; use `bash ./setup.sh` so ZIP execute bits are irrelevant.
- Downloads must contain complete reviewed source. Build seven deterministic ZIPs
  from explicit inventories, including licenses; omit installed/generated/local files.
- Verification and release smoke depended on full programs inside Markdown. Verify
  the built ZIP consumers and canonical Go example source instead.
- Dark-theme links had insufficient contrast. Use the existing light teal palette
  for links throughout the documentation.

## Review loops

1. Inspect every page by reader task, code role, prerequisites and next destination.
   Reuse shared-text review only after comparing the target-specific differences.
2. Rewrite start/example pages and repair their callers, indexes and READMEs.
3. Generate/download/unpack seven examples; compile and run them. The first run passed.
4. Recheck rewritten navigation. A removed historical index anchor failed the built
   route check; restore the anchors and rerun the check.
5. Exercise the exact Go starter command: the Todo document requires `--with server`.
   Restore the flag and explanation in first-use, generation and selection examples.
6. Verify final pairing, formatting, routes, source archives, consumers and release smoke.

## Verification

- `devtools run format:docs` and `format-check:docs`.
- `devtools run build:docs` and `verify:docs`: Vue/TypeScript, built routes and
  historical anchors, search scopes, all seven built ZIP consumers, documentation tools.
- Source inventory tests compare every archived file and license to canonical source
  and check byte-for-byte repeatability.
- Go ZIP consumers run with empty module caches, network disabled and CGO disabled.
- TypeScript consumers use the existing pinned 7.0.2 verification compiler and Node types.
- `devtools run build:dev`, then native `scripts/release/smoke-go.sh`: generate and
  run the canonical Go Todo consumer without extracting code from a guide.
- Browser: both locales/SDK first-use pages at 1280px and 390px, correct ZIP paths,
  no page-level horizontal overflow; all seven ZIP URLs return HTTP 200.
- Optional AI provider integration remains application-owned and is not executed
  against an external model. The downloaded SSE consumers run without provider credentials.

## Page coverage

All rows were reviewed. Unchanged lookup pages retain relevant, compact examples;
review does not require rewriting already useful content.

| Page | Status |
| --- | --- |
| `docs/index.md` | Reviewed |
| `docs/ko/index.md` | Reviewed |
| `docs/typescript/examples/ai-streaming.md` | Reviewed |
| `docs/typescript/examples/index.md` | Reviewed |
| `docs/typescript/examples/webhook-server.md` | Reviewed |
| `docs/typescript/guide/client.md` | Reviewed |
| `docs/typescript/guide/files-links-streams.md` | Reviewed |
| `docs/typescript/guide/generate.md` | Reviewed |
| `docs/typescript/guide/getting-started.md` | Reviewed |
| `docs/typescript/guide/inspect.md` | Reviewed |
| `docs/typescript/guide/models.md` | Reviewed |
| `docs/typescript/guide/named-clients.md` | Reviewed |
| `docs/typescript/guide/remote-inputs.md` | Reviewed |
| `docs/typescript/guide/schema-vocabularies.md` | Reviewed |
| `docs/typescript/guide/selection-cost.md` | Reviewed |
| `docs/typescript/guide/selective-client.md` | Reviewed |
| `docs/typescript/guide/server.md` | Reviewed |
| `docs/typescript/guide/transport.md` | Reviewed |
| `docs/typescript/guide/troubleshooting.md` | Reviewed |
| `docs/typescript/index.md` | Reviewed |
| `docs/typescript/playground.md` | Reviewed |
| `docs/typescript/reference/capabilities.md` | Reviewed |
| `docs/typescript/reference/cli.md` | Reviewed |
| `docs/typescript/reference/client-api.md` | Reviewed |
| `docs/typescript/reference/compatibility.md` | Reviewed |
| `docs/typescript/reference/extensions.md` | Reviewed |
| `docs/typescript/reference/index.md` | Reviewed |
| `docs/typescript/reference/server-api.md` | Reviewed |
| `docs/typescript/reference/streaming.md` | Reviewed |
| `docs/typescript/reference/types.md` | Reviewed |
| `docs/golang/examples/ai-streaming.md` | Reviewed |
| `docs/golang/examples/index.md` | Reviewed |
| `docs/golang/examples/webhook-server.md` | Reviewed |
| `docs/golang/guide/client.md` | Reviewed |
| `docs/golang/guide/files-links-streams.md` | Reviewed |
| `docs/golang/guide/generate.md` | Reviewed |
| `docs/golang/guide/getting-started.md` | Reviewed |
| `docs/golang/guide/inspect.md` | Reviewed |
| `docs/golang/guide/models.md` | Reviewed |
| `docs/golang/guide/named-clients.md` | Reviewed |
| `docs/golang/guide/remote-inputs.md` | Reviewed |
| `docs/golang/guide/schema-vocabularies.md` | Reviewed |
| `docs/golang/guide/selection-cost.md` | Reviewed |
| `docs/golang/guide/selective-client.md` | Reviewed |
| `docs/golang/guide/server.md` | Reviewed |
| `docs/golang/guide/transport.md` | Reviewed |
| `docs/golang/guide/troubleshooting.md` | Reviewed |
| `docs/golang/index.md` | Reviewed |
| `docs/golang/playground.md` | Reviewed |
| `docs/golang/reference/capabilities.md` | Reviewed |
| `docs/golang/reference/cli.md` | Reviewed |
| `docs/golang/reference/client-api.md` | Reviewed |
| `docs/golang/reference/compatibility.md` | Reviewed |
| `docs/golang/reference/extensions.md` | Reviewed |
| `docs/golang/reference/index.md` | Reviewed |
| `docs/golang/reference/server-api.md` | Reviewed |
| `docs/golang/reference/streaming.md` | Reviewed |
| `docs/golang/reference/types.md` | Reviewed |
| `docs/ko/typescript/examples/ai-streaming.md` | Reviewed |
| `docs/ko/typescript/examples/index.md` | Reviewed |
| `docs/ko/typescript/examples/webhook-server.md` | Reviewed |
| `docs/ko/typescript/guide/client.md` | Reviewed |
| `docs/ko/typescript/guide/files-links-streams.md` | Reviewed |
| `docs/ko/typescript/guide/generate.md` | Reviewed |
| `docs/ko/typescript/guide/getting-started.md` | Reviewed |
| `docs/ko/typescript/guide/inspect.md` | Reviewed |
| `docs/ko/typescript/guide/models.md` | Reviewed |
| `docs/ko/typescript/guide/named-clients.md` | Reviewed |
| `docs/ko/typescript/guide/remote-inputs.md` | Reviewed |
| `docs/ko/typescript/guide/schema-vocabularies.md` | Reviewed |
| `docs/ko/typescript/guide/selection-cost.md` | Reviewed |
| `docs/ko/typescript/guide/selective-client.md` | Reviewed |
| `docs/ko/typescript/guide/server.md` | Reviewed |
| `docs/ko/typescript/guide/transport.md` | Reviewed |
| `docs/ko/typescript/guide/troubleshooting.md` | Reviewed |
| `docs/ko/typescript/index.md` | Reviewed |
| `docs/ko/typescript/playground.md` | Reviewed |
| `docs/ko/typescript/reference/capabilities.md` | Reviewed |
| `docs/ko/typescript/reference/cli.md` | Reviewed |
| `docs/ko/typescript/reference/client-api.md` | Reviewed |
| `docs/ko/typescript/reference/compatibility.md` | Reviewed |
| `docs/ko/typescript/reference/extensions.md` | Reviewed |
| `docs/ko/typescript/reference/index.md` | Reviewed |
| `docs/ko/typescript/reference/server-api.md` | Reviewed |
| `docs/ko/typescript/reference/streaming.md` | Reviewed |
| `docs/ko/typescript/reference/types.md` | Reviewed |
| `docs/ko/golang/examples/ai-streaming.md` | Reviewed |
| `docs/ko/golang/examples/index.md` | Reviewed |
| `docs/ko/golang/examples/webhook-server.md` | Reviewed |
| `docs/ko/golang/guide/client.md` | Reviewed |
| `docs/ko/golang/guide/files-links-streams.md` | Reviewed |
| `docs/ko/golang/guide/generate.md` | Reviewed |
| `docs/ko/golang/guide/getting-started.md` | Reviewed |
| `docs/ko/golang/guide/inspect.md` | Reviewed |
| `docs/ko/golang/guide/models.md` | Reviewed |
| `docs/ko/golang/guide/named-clients.md` | Reviewed |
| `docs/ko/golang/guide/remote-inputs.md` | Reviewed |
| `docs/ko/golang/guide/schema-vocabularies.md` | Reviewed |
| `docs/ko/golang/guide/selection-cost.md` | Reviewed |
| `docs/ko/golang/guide/selective-client.md` | Reviewed |
| `docs/ko/golang/guide/server.md` | Reviewed |
| `docs/ko/golang/guide/transport.md` | Reviewed |
| `docs/ko/golang/guide/troubleshooting.md` | Reviewed |
| `docs/ko/golang/index.md` | Reviewed |
| `docs/ko/golang/playground.md` | Reviewed |
| `docs/ko/golang/reference/capabilities.md` | Reviewed |
| `docs/ko/golang/reference/cli.md` | Reviewed |
| `docs/ko/golang/reference/client-api.md` | Reviewed |
| `docs/ko/golang/reference/compatibility.md` | Reviewed |
| `docs/ko/golang/reference/extensions.md` | Reviewed |
| `docs/ko/golang/reference/index.md` | Reviewed |
| `docs/ko/golang/reference/server-api.md` | Reviewed |
| `docs/ko/golang/reference/streaming.md` | Reviewed |
| `docs/ko/golang/reference/types.md` | Reviewed |

## Follow-up: call results and request-stream responsibilities

The initial pass simplified code but omitted recognizable returned values in several
examples. Its request-stream example also fabricated Todo state changes in a client
without showing a receiving server. Adding a status print did not explain that use case.

Review scope: executable API calls in both locales across all 56 canonical SDK pages
(112 localized pages), root README, and linked runnable examples. Type-only declarations,
configuration snippets and generation commands are reviewed for their described artifacts;
they do not need fabricated console output. Keep tested output distinct from illustrative
server responses, and keep complete runnable setup in downloads.

- Ordinary and raw calls: show Todo/list shapes, status/header output and named-client
  Product results. Explain that selection retains response shapes and optional calls can
  return undefined without a request.
- Streams: show item values, response metadata and SSE output. Replace synthetic Todo
  state generation with a client uploading an NDJSON file to a separate Node HTTP server.
  The server logs received titles and returns `{ "imported": 2 }`; the client prints
  `Imported 2 todos`. Include the complete source as the eighth downloadable application.
- Inbound calls: pair request bodies with handler logs and sender-visible status/body.
  Identify the secured router example that returns 401 before authentication is installed.
- Transport, errors, metadata, dynamic validation and envelope mapping: explain their
  observable result or state that configuration alone sends no request.
- No generated SDK runtime behavior or external dependencies change.

Review loop: the first built-link check found NFC Korean fragments that did not match
VitePress's NFD heading IDs. Add a stable explicit request-stream anchor, preserve the old
heading anchor, repair callers and repeat built-route validation. Rechecking selection snippets
also caught an imported feature array named `tasks` colliding with the new result variable;
rename the feature import to `taskOperations` in both locales. The eighth ZIP is included
in the existing exact-source/determinism tests and generated-consumer execution checks.

Validation: `build:docs` and `verify:docs` passed after the anchor fix, including 8 built ZIP
consumers (SDK generation, compilation and execution), both locales, historical anchors,
search scope and documentation tool tests. Final format/build and built-route rechecks also
passed after the last snippet/prose fixes; example runtime evidence remains applicable to
unchanged source.
