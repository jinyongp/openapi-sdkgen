package generator

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Selection names exact OpenAPI operation IDs and METHOD/path route keys.
// The two namespaces form a union. A non-nil empty selection is invalid.
type Selection struct {
	Operations []string `json:"operations,omitempty"`
	Routes     []string `json:"routes,omitempty"`
}

// Canonical returns an independent, sorted and deduplicated selector set.
func (selection *Selection) Canonical() (*Selection, error) {
	if selection == nil {
		return nil, nil
	}
	if len(selection.Operations)+len(selection.Routes) == 0 {
		return nil, fmt.Errorf("selection must include at least one operation or route")
	}
	for _, id := range selection.Operations {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("selection operationId must not be blank")
		}
	}
	for _, route := range selection.Routes {
		method, path, exists := strings.Cut(route, " ")
		if !exists || method == "" || !strings.HasPrefix(path, "/") || strings.ContainsFunc(route, func(r rune) bool { return unicode.IsSpace(r) && r != ' ' }) || strings.Contains(path, " ") {
			return nil, fmt.Errorf("selection route %q must be an exact METHOD and OpenAPI path separated by one space", route)
		}
		for _, r := range method {
			if !strings.ContainsRune("!#$%&'*+-.^_`|~", r) && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
				return nil, fmt.Errorf("selection route %q has an invalid HTTP method", route)
			}
		}
	}
	return &Selection{Operations: canonicalSelectionNames(selection.Operations), Routes: canonicalSelectionNames(selection.Routes)}, nil
}

func canonicalSelectionNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	n := 0
	for _, value := range result {
		if n == 0 || result[n-1] != value {
			result[n] = value
			n++
		}
	}
	return result[:n]
}
