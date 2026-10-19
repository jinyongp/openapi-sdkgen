// Package goruntime exposes the explicit source inventory used by the Go target.
package goruntime

import "embed"

// Sources are selected by the target's explicit dependency manifest. Tests and
// preparation tools are not embedded in the generator or emitted into SDKs.
//
//go:embed ecmaregexp/program.go ecmaregexp/ranges.go ecmaregexp/text.go ecmaregexp/match.go ecmaregexp/scratch.go ecmaregexp/compile.go ecmaregexp/classes.go
//go:embed unicode17/ranges.go unicode17/fold.go unicode17/fold_data.go unicode17/properties.go unicode17/properties_data.go unicode17/normalization.go unicode17/normalization_data.go unicode17/idna.go unicode17/idna_data.go unicode17/LICENSE.txt
//go:embed urlstd/url.go urlstd/host.go urlstd/idna.go urlstd/punycode.go urlstd/encoding.go
var Sources embed.FS
