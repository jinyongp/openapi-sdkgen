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

inspect-benchmark corpus output:
    bash ./scripts/agent/inspect-benchmark {{quote(corpus)}} {{quote(output)}}

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

compatibility-fetch manifest='test/compatibility/holdout.json' corpus='.tmp/compatibility-holdout':
    bash ./scripts/agent/compatibility-fetch {{quote(manifest)}} {{quote(corpus)}} fresh

compatibility-verify manifest='test/compatibility/holdout.json' corpus='.tmp/compatibility-holdout':
    bash ./scripts/agent/compatibility-fetch {{quote(manifest)}} {{quote(corpus)}} offline

compatibility-benchmark manifest='test/compatibility/holdout.json' corpus='.tmp/compatibility-holdout' output='.tmp/compatibility-benchmark.json' typecheck_timeout='3m' document='' selection='' addon='':
    bash ./scripts/agent/compatibility-benchmark {{quote(manifest)}} {{quote(corpus)}} {{quote(output)}} {{quote(typecheck_timeout)}} {{quote(document)}} {{quote(selection)}} {{quote(addon)}}

[positional-arguments]
compatibility-merge manifest output +reports:
    bash ./scripts/agent/compatibility-merge "$@"

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

[positional-arguments]
ts-declarations *args:
    bash ./scripts/agent/ts-declarations "$@"

ts-declarations-test:
    bash ./scripts/agent/ts-declarations-test

ts-declarations-options:
    bash ./scripts/agent/ts-declarations-options

ts-declarations-benchmark report='.tmp/declaration-benchmark/report.json':
    bash ./scripts/agent/ts-declarations-benchmark {{quote(report)}}

ts-typecheck:
    ./scripts/agent/ts-typecheck

ts-compat version='all':
    bash ./scripts/agent/ts-compat {{quote(version)}}

ts-test:
    ./scripts/agent/ts-test

[positional-arguments]
runtime-delivery-check *args:
    bash ./scripts/agent/runtime-delivery-check "$@"

[positional-arguments]
runtime-delivery-browser *args:
    bash ./scripts/agent/runtime-delivery-browser "$@"

[positional-arguments]
runtime-delivery-browser-report *args:
    bash ./scripts/agent/runtime-delivery-browser-report "$@"

[positional-arguments]
runtime-delivery-session *args:
    bash ./scripts/agent/runtime-delivery-session "$@"

[positional-arguments]
runtime-delivery-session-report *args:
    bash ./scripts/agent/runtime-delivery-session-report "$@"

[positional-arguments]
sdk-delivery-check *args:
    bash ./scripts/agent/sdk-delivery-check "$@"

[positional-arguments]
named-clients-check *args:
    bash ./scripts/agent/named-clients-check "$@"

[positional-arguments]
sdk-delivery-browser *args:
    bash ./scripts/agent/sdk-delivery-browser "$@"

[positional-arguments]
sdk-delivery-browser-report *args:
    bash ./scripts/agent/sdk-delivery-browser-report "$@"

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

npm-source-check:
    ./scripts/agent/npm-source-check
