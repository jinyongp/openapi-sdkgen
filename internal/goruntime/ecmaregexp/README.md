# Owned ECMAScript Unicode pattern compiler

`Compile`/`CompileContext` accept a pattern source with the SDK's fixed Unicode
`u` semantics. Flags `i`, `m`, `s` can be scoped in the pattern. The compiler
owns syntax, capture/reference resolution, Unicode class construction and a
flat versioned instruction/edge/range program. It does not execute JavaScript.
The match executor is a separate work unit and is not implemented here yet.

Patterns and inputs use the lossless wire layer's WTF-8 representation for lone
surrogates. Compiler stacks are explicit; deeply nested groups do not recurse on
the Go stack. Repeated class source/flags share immutable range data. Unicode
properties and simple folding use the owned `unicode17` package.

Pinned constructor-pattern cases are prepared with:

```sh
devtools run -- node scripts/goruntime-data/extract-regexp-syntax.mjs
```

Preparation needs the existing development TypeScript 6 API and cached pinned
Test262 source. SDK consumers and ordinary conformance tests do not need it.
The fixture contains 3,930 unique static `u` sources from 710 of 2,119 source
files, including 324 invalid patterns. It recompiles patterns; it does not claim
the original JS tests passed. Per-file pending dynamic/API-scope coverage stays
visible in `testdata/goruntime/regexp-coverage.json` for the matcher integration.
