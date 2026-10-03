# Reference and sequential media regressions

Baseline: `3d3b1b1facf2f69ed16a12a7a83a8dce49068141`.

| Input | Baseline | Acceptance after implementation |
| --- | --- | --- |
| schema-locations.json | target E501/E507 | generate, strict types, validate referenced string |
| schema-array.json | normalization E130 | generate and validate referenced array member |
| schema-percent.json | normalization E130 | decode URI fragment before JSON Pointer tokens |
| sequential-untyped.json | target E502 | buffered unknown and raw bytes; incremental API requires itemSchema |
| sse-event.json | generate; default JSON payload mapping fails Event schema | buffered array and incremental Event mapping; request/server round trip |
| links/root.json | external operationRef helper omitted | mounted file closure, target server/path/parameters, cyclic helper references |

The reference probes also run with OpenAPI 3.0.3, 3.1.1 and 3.2.0. Native
boolean schemas and resource scope apply to 3.1/3.2; 3.0 follows the compiler's
compatibility normalization and unsupported dialect diagnostics.

`schema` describes the complete buffered value. `itemSchema` describes one
incremental value. JSON application data uses an explicit stream adapter.

Run generation and strict checking with `devtools run generate:check INPUT
typescript -- --diagnostic-mode collect`. Runtime probes live in the TypeScript
target tests and run through `devtools run dev:test` and `devtools run verify:typescript`.
