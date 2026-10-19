# Go runtime conformance inputs

`corpora.json` records immutable upstream commits, paths and hashes. It inventories
2,119 RegExp syntax/matcher candidate tests and their metadata; `pending` means
adaptation/semantic classification has not run. It is not a passing conformance result.
JS object APIs and non-`u` modes must receive explicit reasons before exclusion;
unsupported engine syntax must not be excluded to obtain a passing suite.

Run `devtools run -- python3 scripts/goruntime-data/fetch-corpora.py --output .tmp/goruntime-corpora`
only when preparing fixtures. Ordinary tests use committed normalized fixtures and
never download upstream code. Original JS files and harness are cached separately
for review and adaptation. `url/` retains WPT parsing/IDNA/encoding expectations;
See `url/SOURCE_README.md` for the original URL dataset format.
Setter/DOM JS sources remain inventoried but are not part of the parser API.
Upstream licenses are retained in `licenses/`; the Test262 notice only has trailing
whitespace normalized. Original notice bytes remain identified in `corpora.json`.

Existing product regression inputs remain canonical at
`internal/target/golang/testdata/regex-cases.json` (42 cases) and
`plans/golang-target/format-probe/url_test.go` (28 cases). Their earlier results do
not establish full ECMAScript/WHATWG conformance.
