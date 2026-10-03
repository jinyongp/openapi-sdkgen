# AGENTS.md

## Commands

Use `devtools run PURPOSE:COMMAND ARG...` for local project work. Devtools selects this
worktree, environment, ports, and the local Node version. Discover commands with
`devtools command list`; inspect one with `devtools command inspect NAME` and
`devtools run PURPOSE:COMMAND --help`. Script ownership and output policy are described
in [scripts/README.md](scripts/README.md).

CI prepares the required toolchain and calls the same SSOT scripts directly.
Task composition calls scripts directly and does not reselect the environment.
Command names use the namespaces listed in [scripts/README.md](scripts/README.md).
Arguments follow the command name directly; a devtools separator is unnecessary.

```txt
devtools run dev:build
devtools run dev:test
devtools run dev:ci [go|typescript-runtime|typescript-tools|compiler VERSION]
devtools run dev:check
devtools run generate:sdk INPUT OUTPUT [TARGET]
devtools run generate:check INPUT [TARGET] [-- GENERATOR_OPTIONS...]
devtools run verify:typescript
devtools run verify:compilers [all|5.7.3|5.9.3|6.0.3|7.0.2]
devtools run dev:coverage
devtools run verify:typescript coverage
```

Ordinary CI checks each suite once. `dev:check` adds cross-builds and example clients.
Coverage and performance measurement are optional commands, not fixed percentage
or literal-source gates in ordinary CI.

## TypeScript declarations

TypeScript source and generated TypeScript use named interfaces or type aliases
for object shapes, explicit function return types (including callbacks), and
explicit variable types. Keep inference at language boundaries where annotations
are unavailable, such as `for...of` bindings; preserve exact public literal types.
Give each internal object contract one canonical interface or type alias. Import
and reuse it, or derive a view with indexed, mapped, or utility types. Avoid
copying its fields into another declaration; separate public schema identities
keep their own contracts.

## Documentation

Korean documentation uses natural Korean for general concepts in prose, headings,
navigation, tables, example comments, and page UI. Exact code identifiers,
commands, and literal values use inline code; product and standard names keep
their official spelling. Review each Korean page as native writing.

```txt
devtools run docs:install
devtools run docs:validate
devtools run docs:build
devtools run docs:dev
devtools run docs:preview
devtools run docs:lock
```

Use `devtools process start docs:dev --request-id UUID` for a managed server and wait for readiness
before using it. Stop the returned execution when finished.

## Release

```txt
devtools run release:publish [--dry-run|-n] [--yes|-y] [--since TAG] [--resume TAG] [patch|minor|major|vX.Y.Z[-prerelease]]
```

Release requires clean `main`, validates the existing HEAD, and atomically pushes
`main` and an annotated tag. Dry runs validate without editing files, creating
commits or publishing. Failed checks preserve HEAD; failed pushes remove the
local tag created by that attempt. `--resume` dispatches the existing tag's workflow.
Releaseway standard generates GitHub release notes. Add breaking-change and
migration explanations there; retries preserve published release bodies.

## Output

Successful commands show the essential result; failures show the actual cause
and bounded diagnostics under `.tmp/runs/`. Human explanations use stderr;
JSON and exports use stdout. Logs and terminal output redact credentials before
storage/display. `--verbose` requests full redacted output for one invocation
and warns on stderr before work starts. Prefer default commands for routine work.
