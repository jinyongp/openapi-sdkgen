package generator

import (
	"fmt"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GoOptions describes the consumer-owned destination of generated Go source.
// ImportPath is explicit: generation never discovers or changes a go.mod file.
type GoOptions struct {
	Package    string `json:"package"`
	ImportPath string `json:"importPath"`
}

// CanonicalGoOptions returns an independent, validated Go configuration.
func CanonicalGoOptions(options *GoOptions) (GoOptions, error) {
	value := GoOptions{Package: "api"}
	if options != nil {
		value = *options
		if value.Package == "" {
			value.Package = "api"
		}
	}
	first, _ := utf8.DecodeRuneInString(value.Package)
	if !token.IsIdentifier(value.Package) || unicode.IsUpper(first) || value.Package == "_" || value.Package == "main" {
		return GoOptions{}, fmt.Errorf("invalid Go package %q: use an unexported non-main package identifier", value.Package)
	}
	if value.ImportPath == "" {
		return GoOptions{}, fmt.Errorf("--go-import-path is required for the golang target")
	}
	if strings.HasPrefix(value.ImportPath, "-") {
		return GoOptions{}, fmt.Errorf("invalid Go import path %q", value.ImportPath)
	}
	for _, part := range strings.Split(value.ImportPath, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.EqualFold(part, "vendor") {
			return GoOptions{}, fmt.Errorf("invalid Go import path %q", value.ImportPath)
		}
		stem, _, _ := strings.Cut(part, ".")
		reserved := strings.ToUpper(stem)
		if reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" || len(reserved) == 4 && (strings.HasPrefix(reserved, "COM") || strings.HasPrefix(reserved, "LPT")) && reserved[3] >= '1' && reserved[3] <= '9' {
			return GoOptions{}, fmt.Errorf("invalid Go import path %q: reserved Windows path element", value.ImportPath)
		}
		if index := strings.LastIndexByte(part, '~'); index >= 0 {
			tail := part[index+1:]
			if tail != "" && strings.Trim(tail, "0123456789") == "" {
				return GoOptions{}, fmt.Errorf("invalid Go import path %q: Windows short path element", value.ImportPath)
			}
		}
		for _, char := range part {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-._~+", char)) {
				return GoOptions{}, fmt.Errorf("invalid Go import path %q", value.ImportPath)
			}
		}
	}
	return value, nil
}
