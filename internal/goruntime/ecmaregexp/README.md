# Owned ECMAScript Unicode pattern runtime

`Compile`/`CompileContext` accept a pattern source with the SDK's fixed Unicode
`u` semantics. Flags `i`, `m`, `s` can be scoped in the pattern. The compiler
owns syntax, capture/reference resolution, Unicode class construction and a
flat versioned instruction/edge/range program. It does not execute JavaScript.
`Program.Match` executes the immutable program with per-call captures and explicit
continuation/backtracking stacks. It does not recurse on the Go call stack.
The complete search shares a 100,000,000-step budget and checks context every
1,024 steps. Budget and context errors remain distinct from a mismatch.

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

The Boolean matching fixture is separately prepared with:

```sh
devtools run -- node scripts/goruntime-data/extract-regexp-matches.mjs
```

It derives 95 unique Boolean contracts from 98 static assertions in 19 pinned
Test262 files. These expectations come from the assertions, not the new engine.
Capture values are checked through backreferences and assertions in additional
independent and Node differential cases. Dynamic JS/API coverage in the syntax
manifest remains pending for integration; this is not a full Test262 pass.

Inputs are scanned by byte cursor; simple repetitions use compact retry state.
A cache retains at most 32 exclusively borrowed fixed-size scratch buffers. Return
clears input, program, context and grown buffers. The arrays hold only integers.
Pool entries are hints over a fixed 32-buffer arena; atomic lease ownership guards
against stale entries and recovers buffers after pool GC. Overflow state is not
retained. This avoids a single global channel in concurrent matches. Concurrent
calls share no match state.

Node 24.22's scoped case modifier has a narrow oracle discrepancy for Kelvin
sign/long s literals. ECMA-262 Canonicalize with the locally updated flags and
Unicode 17 simple folding require these literals to match `k`/`s`. Independent
contracts retain those results; the matrix's Kelvin expectation is explicitly
corrected rather than silently adopting Node's result. See the W05 implementation
record and evidence for the reproduction and normative references.

Supplementary dynamic inputs can be prepared with:

```sh
devtools run -- node scripts/goruntime-data/trace-regexp-matches.mjs
```

This runs hash-pinned test/harness source only during preparation in disposable
Node VM contexts. It records 1,919 fixed-u Boolean traces from 477 source files
and retains all 2,119 file outcomes in `regexp-trace-coverage.json`. Node is the
supplementary oracle; these are not independent assertions or a full Test262
pass. Sticky/nonzero-lastIndex and JS object/coercion behavior are outside the
SDK matching contract. Inputs over 8,192 UTF16 units and patterns over 32,768
units are not traced; these are preparation limits only. The corpus retains
1,863 oversized calls. Exhaustive Unicode boundaries and the independent 1MiB
matching case cover those areas separately. Twenty-two JS runs are incomplete:
realm/descriptor/coercion API assertions and four non-u lexical enumeration
tests; their exact errors remain in the manifest and are not counted as passes.
