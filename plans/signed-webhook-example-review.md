# Signed webhook applications

## Scope and boundaries

Separate TS/Go senders sign raw order-event bytes and deliver them to local HTTP
receivers. Receiver middleware verifies ID, timestamp and HMAC before passing the
original bytes to generated webhook handlers for schema validation. Senders query
the resulting processed-order list with generated clients. Both use only platform
libraries and the same contract. This example's HMAC protocol is explicitly local;
external providers retain their own signing rules.

Sign `id.timestamp.rawBody` with HMAC-SHA256, encoded as `v1=<lowercase hex>`.
Accept only single-value signed headers, restricted IDs, numeric Unix seconds
within ±300 seconds and exactly 32-byte signatures. Comparison is constant-time.
Bodies are limited to 64 KiB. Verification precedes JSON decoding and all effects.

Validated duplicate event data returns 204 without a second effect, including a
properly re-signed retry with different whitespace/timestamp. A reused ID with
different data returns 409. Failed signature/schema checks do not reserve IDs.
Check-and-record is synchronous in TS and mutex-protected in Go. Receipt state
and orders are in memory and are lost on receiver restart; no durable processing
guarantee is claimed.

## Review coverage

| Area | Evidence |
| --- | --- |
| Wire signing | Independent Node crypto oracle exercises both native implementations |
| Original bytes and signed fields | Body whitespace, ID and timestamp tampering fail with no state changes |
| Freshness and signature shape | Stale/future timestamps and malformed signatures return 401 |
| Generated decoder boundary | Unsigned malformed JSON returns 401; signed malformed/schema-invalid bodies return 400 |
| Duplicate effects | Five simultaneous retries add exactly one order; re-signed equivalent data is accepted; conflicting data returns 409 |
| Failure recovery | Invalid event ID remains usable by a later valid event; oversize returns 413 |
| Public flow | Separate sender/receiver programs; generated client reads order state; paired docs, setup and complete source ZIPs |

The review made signed-header cardinality explicit in the Go wrapper, preserved
snapshot-before-network-write in state queries, and removed a TS secret type
assertion by capturing validated configuration in a definite string binding.

## Validation

Final rerun passed after the re-signed retry assertion and documentation snippet
alignment: `devtools run build:docs`, `devtools run verify:docs`,
`devtools run fmt-check:dev`, and `git diff --check`. Verification passed all
fourteen archive consumers and their HTTP checks, docs types/routes/paired locales,
archive inventories/determinism and tools. No external provider calls are required.

All four new README setup scripts ran with an explicit local `SDKGEN_BIN` and
frozen dependency locks. The Go signed receiver was built with `-race` and the
shared checks, including five simultaneous duplicate deliveries, passed without
race reports using `.tmp/verify-signed-webhook-race.mjs`. Verification ran in WSL.
