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
| Provider evidence | pending | pending | pending | pending |
| Combined review | pending | pending | pending | pending |

Each unit requires relevant generated SDK checks, caller review and no outstanding
confirmed defects before committing. Final checks apply to the complete revision.
