# Background report applications

## Scope and choices

Add one complete application for each SDK target. Separate HTTP server processes
read bundled orders, aggregate one month by region, expose queued/running/completed/
cancelled jobs and serve the computed CSV. Generated clients submit, poll, download
and save October's report, then cancel a second job. Both use the same OpenAPI
contract and sample orders. Go has no external module dependencies; TypeScript
uses Node built-ins with the existing pinned compiler toolchain.

Jobs belong to the server, independent of an individual HTTP request. Cancelling
client polling does not cancel a job; cancellation requires its explicit endpoint.
Queued/running jobs may be cancelled, repeated cancellation is safe, completed
jobs reject cancellation with 409, and unfinished/cancelled downloads return 409.
Unknown job IDs return 404. Empty months complete with a header-only report.

The server stores jobs in memory and loses them on restart. It pauses 200 ms between
records to make progress visible with the small input. Clients use ten-second
deadlines. Durable queues, application authentication and other suggested examples
(file conversion/signed webhooks) remain outside this work unit.

## Review coverage

| Area | Review and evidence |
| --- | --- |
| Contract and generated APIs | Identical contracts, schema-derived job types, all four operations used by each compiled client |
| Computation and file result | Real month filter and regional totals; October, September and empty-month cases; saved client file checked byte-for-byte |
| Worker lifetime and cancellation | Queued and running cancellation; repeated cancellation; stopped worker must not advance after cancellation |
| Go shared state | Job transitions protected by a mutex; response snapshots release the lock before network writes |
| Error paths | Invalid JSON/month/trailing JSON, unknown IDs, premature/cancelled downloads and completed cancellation |
| Documentation and downloads | Four locale/SDK pages, sidebar/catalog, example indexes, source archive allowlists and README commands |

Review found the Go handler held the job-state mutex while writing HTTP responses.
Copy the job/file under the lock and release it before I/O, so a slow reader cannot
block all workers and cancellation. Recheck both handlers through built downloads.
The first full documentation check found a historical hardcoded topic count; retain
ID uniqueness and historical route/anchor protections while allowing a new topic.

## Validation

Passed after review fixes: `devtools run build:docs`, `devtools run verify:docs`,
`devtools run fmt-check:dev`, and `git diff --check`. The verification suite runs
all ten built example archives plus documentation types, routes, paired locales,
archive contents/determinism and tool tests. Both README setup scripts also ran
with an explicit local `SDKGEN_BIN`. The downloaded archive verifier executes
both separate servers and generated consumers, checks saved CSV bytes, and runs
the same lifecycle/error assertions against both servers. No external service is
needed for this example. Existing AI verification still uses real provider SDKs
against its test-only provider boundary; it does not make paid calls.

The Go report server was additionally built with `-race`; two concurrent executions
of the lifecycle/error checks passed with no race reports. The disposable harness
was `.tmp/verify-report-race.mjs`, calling the same committed report checks against
the race-enabled executable. Validation ran in WSL with the project toolchain.
