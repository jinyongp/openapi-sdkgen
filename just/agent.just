set quiet := true
set no-cd := true

[private]
default:
    @just --list agent

check:
    ./scripts/agent/check

ci:
    bash ./scripts/agent/ci

fmt:
    ./scripts/agent/fmt

fmt-check:
    ./scripts/agent/fmt-check

vet:
    ./scripts/agent/vet

vuln:
    bash ./scripts/agent/vuln

vuln-test:
    bash ./scripts/agent/vuln-test

fuzz:
    bash ./scripts/agent/fuzz

race:
    bash ./scripts/agent/race

test:
    ./scripts/agent/test

build:
    ./scripts/agent/build

mod-tidy:
    ./scripts/agent/mod-tidy

mod-tidy-check:
    ./scripts/agent/mod-tidy-check

mod-verify:
    ./scripts/agent/mod-verify

generate input output target='typescript':
    ./scripts/agent/generate {{quote(input)}} {{quote(output)}} {{quote(target)}}

[positional-arguments]
generate-check input target='typescript' *args:
    ./scripts/agent/generate-check "$@"

generate-check-test:
    ./scripts/agent/generate-check-test

diagnostic-harvest manifest output:
    bash ./scripts/agent/diagnostic-harvest {{quote(manifest)}} {{quote(output)}}

conformance:
    ./scripts/agent/conformance

[positional-arguments]
representation-check *args:
    bash ./scripts/agent/representation-check "$@"

[positional-arguments]
representation-artifacts *args:
    bash ./scripts/agent/representation-artifacts "$@"

[positional-arguments]
representation-migration *args:
    bash ./scripts/agent/representation-migration "$@"

[positional-arguments]
representation-measure *args:
    bash ./scripts/agent/representation-measure "$@"

coverage:
    ./scripts/agent/coverage

perf:
    ./scripts/agent/perf

identifier-perf:
    bash ./scripts/agent/identifier-perf

perf-profile workload='self-contained' phase='emit':
    ./scripts/agent/perf-profile {{quote(workload)}} {{quote(phase)}}

perf-publication benchtime='3x' count='5':
    bash ./scripts/agent/perf-publication {{quote(benchtime)}} {{quote(count)}}

perf-acceptance:
    ./scripts/agent/perf-acceptance

ts-lock:
    ./scripts/agent/ts-lock

ts-install:
    ./scripts/agent/ts-install

ts-fmt:
    ./scripts/agent/ts-fmt

ts-fmt-check:
    ./scripts/agent/ts-fmt-check

ts-lint:
    ./scripts/agent/ts-lint

ts-typecheck:
    ./scripts/agent/ts-typecheck

ts-test:
    ./scripts/agent/ts-test

typescript-split-diff baseline:
    ./scripts/agent/typescript-split-diff {{quote(baseline)}}

example-todo:
    ./scripts/agent/example-todo

example-advanced:
    ./scripts/agent/example-advanced

example-capabilities:
    ./scripts/agent/example-capabilities

release-check:
    ./scripts/agent/release-check

release-script-test:
    ./scripts/agent/release-script-test

release-workflow-test:
    ./scripts/agent/release-workflow-test

npm-package version:
    ./scripts/agent/npm-package {{quote(version)}}

npm-package-check package_dir='.tmp/npm-package':
    ./scripts/agent/npm-package-check {{quote(package_dir)}}

npm-publish package_dir dist_tag:
    ./scripts/agent/npm-publish {{quote(package_dir)}} {{quote(dist_tag)}}

npm-publish-test:
    ./scripts/agent/npm-publish-test
