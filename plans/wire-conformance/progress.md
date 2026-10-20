# Wire conformance work

Baseline: `65558c4`. Scope: query send/receive, URLencoded bodies, schema references,
XML/JSON-seq, format assertions, provider evidence and unsupported operation refs.
Preserve documented unsupported features and OpenAPI's ambiguous space/pipe data delimiters.

| Unit | Implementation | Review | Validation | Commit |
| --- | --- | --- | --- | --- |
| Query boundaries | complete | complete: callers, codec precedence, union order and error mapping | TypeScript full; generated Go query/inbound suites passed | query-boundaries commit |
| Form encoding modes | complete | complete: mode presence, style defaults, codec precedence, shared escaping and multipart callers | TypeScript full; generated Go form/query and multipart suites passed | form-encoding commit |
| Reference resources and diagnostics | complete | complete: resource identity, opaque data, source immutability, containment, remote locks, direct and CLI callers | compiler/IR/CLI suites; file/stdin/URL and both diagnostic modes passed | reference-resources commit |
| XML and JSON-seq | complete | complete: XML escaping order/CDATA, JSON whitespace and RS boundaries, NDJSON EOF and cleanup | TypeScript full; generated Go XML independent parser and protocol suites passed | media-conformance commit |
| Format assertions | complete | complete: syntax without URL repair, UTS46/A-label/context/Bidi checks, mailbox grammar, selected runtime dependencies and licensing | 369 pinned official cases in both targets; TypeScript full/lint and Go format/URL/IDNA suites passed | format-assertions commit |
| Provider evidence | complete: pinned authoring preparation, independent Go compilation, source-bound measurement gates and historical manifest preservation | reviewed: containment, opaque data, operation overrides, evidence integrity and failure semantics | preparation/runner/merge and Python/docs evidence tests passed; DigitalOcean TypeScript generated and strictly typechecked all 739 contracts, Go explicitly blocked by its invalid Unicode pattern; Zenith Go generated and compiled 40 routes | `f4d5f63` |
| Provider follow-up | Go rejects empty request media contracts before emission; required/optional cases covered | complete: shared request planner, union and server callers | focused regressions and pinned Spoonacular input passed: explicit input rejection replaces invalid generated code | `6d31919` |
| Combined review | fixed shared serialization layer dependency and Go referenced/allOf query shape handling; malformed query object tokens now fail explicitly | all six areas reviewed; affected query callers rechecked | TypeScript full/runtime boundaries/tools, Go query, vet/format, Python and docs checks passed; complete Go suite exposed three stale assertion/documentation failures; all three rechecked and passed after correction; remote full CI and source-bound corpus measurement pending | pending |

Each unit requires relevant generated SDK checks, caller review and no outstanding
confirmed defects before committing. Final checks apply to the complete revision.
