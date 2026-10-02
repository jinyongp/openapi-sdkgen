# AGENTS.md

## Scope

This repository uses agent-safe command wrappers.

## Command rules

- Use `just agent ...` for project operations.
- Do not run direct `go test`, `go build`, `go run`, `pnpm`, or `npm` commands when an agent wrapper exists.
- Read-only inspection commands such as `sed`, `rg`, `ls`, and `git status` are allowed.

## Agent commands

```txt
just agent check
just agent ci
just agent fmt
just agent fmt-check
just agent vet
just agent vuln
just agent vuln-test
just agent fuzz
just agent race
just agent test
just agent build
just agent inspect-benchmark CORPUS_DIRECTORY REPORT_JSON
just agent mod-tidy
just agent mod-tidy-check
just agent mod-verify
just agent generate INPUT OUTPUT [TARGET]
just agent generate-check INPUT [TARGET] [-- GENERATOR_OPTIONS...]
just agent generate-check-test
just agent compatibility-fetch [MANIFEST] [CORPUS_DIRECTORY]
just agent compatibility-verify [MANIFEST] [CORPUS_DIRECTORY]
just agent compatibility-benchmark [MANIFEST] [CORPUS_DIRECTORY] [OUTPUT] [TYPECHECK_TIMEOUT] [DOCUMENT_ID]
just agent compatibility-merge MANIFEST OUTPUT REPORT...
just agent conformance
just agent representation-check --baseline BINARY --candidate BINARY [OPTIONS]
just agent representation-artifacts --report REPORT [OPTIONS]
just agent representation-migration --baseline BINARY --candidate BINARY [OPTIONS]
just agent representation-measure --report REPORT --artifacts REPORT [OPTIONS]
just agent perf
just agent identifier-perf
just agent perf-profile
just agent perf-acceptance
just agent ts-lock
just agent ts-install
just agent ts-fmt
just agent ts-fmt-check
just agent ts-lint
just agent ts-typecheck
just agent ts-declarations [--runtime DIRECTORY|--generated DIRECTORY|--files LIST_JSON] [--json REPORT_JSON]
just agent ts-declarations-test
just agent ts-declarations-options
just agent ts-declarations-benchmark [REPORT_JSON]
just agent ts-compat [all|5.7.3|5.9.3|6.0.3|7.0.2]
just agent ts-test
just agent runtime-delivery-check [BASELINE_COMMIT]
just agent runtime-delivery-browser
just agent runtime-delivery-browser-report [OBSERVED_SUMMARY_JSON]
just agent runtime-delivery-session [RUNTIME_REPORT]
just agent runtime-delivery-session-report [OBSERVED_SUMMARY_JSON] [SESSION_MANIFEST]
just agent sdk-delivery-check [--sizes 100,1000,10000]
just agent named-clients-check [--graph-source PINNED_OPENAPI_FILE]
just agent sdk-delivery-browser V1_REPORT V2_REPORT
just agent sdk-delivery-browser-report RUN_DIRECTORY OBSERVED_SUMMARY_JSON
just agent typescript-split-diff BASELINE
just agent example-todo
just agent example-advanced
just agent example-capabilities
just agent release-check
just agent release-script-test
just agent npm-source-check
```

## TypeScript declarations

TypeScript source and generated TypeScript use named interfaces or type aliases
for object shapes, explicit function return types (including callbacks), and
explicit variable types. Keep inference at language boundaries where annotations
are unavailable, such as `for...of` bindings; preserve exact public literal types.
Give each internal object contract one canonical interface or type alias. Import
and reuse it, or derive a view with indexed, mapped, or utility types. Avoid
copying its fields into another declaration; separate public schema identities
keep their own contracts.

## Documentation commands

Korean documentation uses natural Korean for general concepts in prose, headings,
navigation, tables, example comments, and page UI. Exact code identifiers, commands,
and literal values use inline code; product and standard names keep their official
spelling. Review each Korean page as native writing, including particles and sentence
structure, rather than translating isolated words.

VitePress uses normal user-facing commands, not `scripts/agent` wrappers:

```txt
just docs install
just docs validate
just docs dev
just docs build
just docs preview
just docs lock
```

## User release command

```txt
just release [patch|minor|major|vX.Y.Z[-prerelease]]
just release -- [--dry-run|-n] [--yes|-y] [--since TAG] [--resume TAG] [patch|minor|major|vX.Y.Z[-prerelease]]
```

`just release` is the user-facing release command, not an agent wrapper. It
shows the commits and release-note base, recommends a conventional-commit bump,
prepares and commits only the dated changelog entry, runs the full agent check
on that HEAD, then atomically pushes `main` and the annotated tag. Dry runs preview
the entry without editing or committing; failed checks preserve the preparation
commit for a same-version retry.

## Output policy

- Agent scripts print short success summaries.
- On failure, scripts print the failing command and a bounded tail of a log under `.tmp/agent-logs/`.
- Use `VERBOSE=1` only for detailed output.
