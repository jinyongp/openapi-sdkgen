# Provider generation comparison

- Earlier Go run versus fresh TypeScript run; not interleaved. Same production source files, pinned inputs and runtime flags; uncontrolled host load may differ.
- Five samples per case; memory is maximum process RSS, not allocations or retained Go heap.
- Both enable server addon; metadata addon disabled. Exact first ten previously emitted Go route keys used on both targets.
- Render/format/hash only, no generated source disk publication, consumer compilation, TypeScript type checking or bundling.
- Time uses GNU time elapsedSeconds for both targets, excluding the 0.5 second monitor polling delay included in the earlier prose table.

## Full generation

| Provider | TypeScript seconds median [min–max] | Go seconds median [min–max] | TypeScript peak RSS MiB | Go peak RSS MiB | Go / TypeScript time | TypeScript source MiB | Go source MiB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| github | 1.75 [1.67–2.52] | 8.97 [8.89–8.99] | 228 | 584 | 5.13× | 33.9 | 156.6 |
| stripe | 1.78 [1.75–1.82] | 10.94 [10.90–11.04] | 242 | 566 | 6.15× | 36.7 | 236.0 |
| cloudflare | 6.00 [5.79–7.19] | 39.44 [39.16–39.74] | 658 | 1960 | 6.57× | 107.8 | 733.3 |
| gitlab | 1.93 [1.79–2.13] | 10.87 [10.86–10.91] | 216 | 635 | 5.63× | 45.3 | 212.0 |
| digitalocean | 5.16 [3.17–6.85] | 3.29 [3.23–3.29] | 492 | 497 | 0.64× | 3.7 | 3.6 |
| twilio | 0.29 [0.27–0.32] | 1.94 [1.92–1.94] | 57 | 114 | 6.69× | 6.4 | 41.1 |
| microsoft-graph-beta | 101.53 [97.31–111.72] | 308.95 [307.16–313.31] | 3683 | 6611 | 3.04× | 2252.4 | 3047.3 |

## Ten exact routes

| Provider | TypeScript seconds median [min–max] | Go seconds median [min–max] | TypeScript peak RSS MiB | Go peak RSS MiB | Go / TypeScript time | TypeScript source MiB | Go source MiB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| github | 0.80 [0.77–0.83] | 1.06 [1.04–1.08] | 192 | 256 | 1.32× | 0.8 | 1.1 |
| stripe | 0.65 [0.64–0.67] | 4.06 [4.05–4.09] | 133 | 310 | 6.25× | 5.9 | 78.2 |
| cloudflare | 2.19 [1.90–2.41] | 3.25 [3.22–3.29] | 445 | 684 | 1.48× | 0.9 | 1.9 |
| gitlab | 0.53 [0.51–1.25] | 1.09 [1.08–1.10] | 147 | 291 | 2.06× | 0.9 | 1.7 |
| digitalocean | 4.64 [3.12–6.13] | 3.03 [2.99–3.06] | 496 | 488 | 0.65× | 0.7 | 0.5 |
| twilio | 0.12 [0.12–0.13] | 0.17 [0.17–0.17] | 43 | 52 | 1.42× | 0.7 | 0.7 |
| microsoft-graph-beta | 9.60 [9.29–9.90] | 19.33 [19.11–19.37] | 1852 | 4053 | 2.01× | 0.8 | 1.1 |
