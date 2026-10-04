# AGENTS.md

## Commands

Use `devtools run COMMAND:TARGET ARG...` for local project work. Devtools selects this
worktree, environment, ports, and the local Node version. Discover commands with
`devtools command list`; inspect one with `devtools command inspect NAME` and
`devtools run COMMAND:TARGET --help`. Script ownership and output policy are described
in [scripts/README.md](scripts/README.md).

CI prepares the required toolchain and calls the same SSOT scripts directly.
Task composition calls scripts directly and does not reselect the environment.
Command names put the operation before the target groups listed in
[scripts/README.md](scripts/README.md).
Arguments follow the command name directly; a devtools separator is unnecessary.

```txt
devtools run build:dev
devtools run test:dev
devtools run ci:dev [go|typescript-runtime|typescript-tools|compiler VERSION]
devtools run check:dev
devtools run sdk:generate INPUT OUTPUT [TARGET]
devtools run check:generate INPUT [TARGET] [-- GENERATOR_OPTIONS...]
devtools run typescript:verify
devtools run compilers:verify [all|5.7.3|5.9.3|6.0.3|7.0.2]
devtools run coverage:dev
devtools run typescript:verify coverage
```

Ordinary CI checks each suite once. `check:dev` adds cross-builds and example clients.
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
devtools run install:docs
devtools run validate:docs
devtools run build:docs
devtools run dev:docs
devtools run preview:docs
devtools run lock:docs
```

Use `devtools process start dev:docs --request-id UUID` for a managed server and wait for readiness
before using it. Stop the returned execution when finished.

## Release

```txt
devtools run publish:release [--dry-run|-n] [--yes|-y] [--since TAG] [--resume TAG] [patch|minor|major|vX.Y.Z[-prerelease]]
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
