# Changelog

## Unreleased

- Generated runtime source now strict-typechecks with TypeScript 5.7.3 and later.
  CI checks pinned 5.7.3, 5.9.3, 6.0.3, and 7.0.2 consumer compilers.

## v9.0.0 — 2026-10-01

- **Breaking:** Generated SSE clients and server inbound handlers use standard
  Event objects by default. `data` stays a string and `event`, `id`, and `retry`
  remain available. Configure an explicit JSON stream adapter to retain the
  previous JSON payload mapping. See [stream migration](docs/reference/streaming.md#regeneration-migration).
- **Breaking:** Sequential `.raw()` responses, including schema-only responses,
  preserve the unconsumed Fetch body and expose `undefined` as `data`. Use the
  ordinary call for decoded data or read `raw.response` for bytes.
- Schema references support arbitrary Schema locations, array indices, and URI
  fragment encoding while preserving named recursive identities. Untyped
  built-in sequential responses generate buffered `unknown` output.

These changes apply when regenerating SDK source. Existing generated source
retains its behavior.
