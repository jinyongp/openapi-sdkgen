# Owned Unicode 17 runtime

The package uses only the Go standard library. The four `*_data.go` files come
from the 15 hash-pinned Unicode inputs in `plans/golang-zero-dependencies/unicode-inputs.json`.
`fetch-unicode.py` explicitly prepares those inputs; `generate-unicode.py` runs
offline. Normal builds and tests do not fetch data.

```sh
devtools run -- python3 scripts/goruntime-data/fetch-unicode.py --output .tmp/goruntime-unicode17
devtools run -- python3 scripts/goruntime-data/generate-unicode.py --input .tmp/goruntime-unicode17 --output internal/goruntime/unicode17
devtools run -- gofmt -w internal/goruntime/unicode17
```

The implementation owns ECMAScript character properties/simple folding and NFC
normalization. IDNA mapping, joining and bidi lookups expose data; the URL parser
owns the UTS #46 policy and validation. Fixed arrays have no map initialization
that would force every data group into a linked consumer.

Official conformance inputs in `testdata/` retain their exact bytes inside gzip
files. Tests check their SHA256 before evaluating normalization/folding/IDNA data.
A supplementary Node Unicode 17 oracle checks all 867 property aliases and range
boundaries/gaps; it skips when Node or the matching Unicode version is unavailable.
The official input checks do not skip. See `LICENSE.txt` for Unicode data terms.
