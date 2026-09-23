set quiet := true
set no-cd := true

# User-facing documentation commands. These intentionally do not use agent
# wrappers: they are the normal local workflow for the VitePress site.
install:
    CI=true corepack pnpm@12.4.1 --dir docs install --frozen-lockfile

lock:
    corepack pnpm@12.4.1 --dir docs clean --lockfile
    corepack pnpm@12.4.1 --dir docs install --lockfile-only

validate:
    corepack pnpm@12.4.1 --dir docs run docs:validate
    corepack pnpm@12.4.1 --dir docs run docs:validate:test

dev:
    corepack pnpm@12.4.1 --dir docs run docs:dev

build:
    corepack pnpm@12.4.1 --dir docs run docs:build

preview:
    corepack pnpm@12.4.1 --dir docs run docs:preview
