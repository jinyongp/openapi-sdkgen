package generator

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Client names an independent generated client entry, sharing source and runtime.
type Client struct {
	Selection *Selection `json:"selection"`
}

var clientNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// CanonicalClients validates portable entry names and exact API assignments.
// A nil map preserves single-client generation; an explicit empty map is invalid.
func CanonicalClients(clients map[string]Client) (map[string]Client, error) {
	if clients == nil {
		return nil, nil
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("clients must include at least one named client")
	}
	result := make(map[string]Client, len(clients))
	for _, name := range sortedClientNames(clients) {
		upper := strings.ToUpper(name)
		reserved := upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" ||
			(len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9')
		if !clientNamePattern.MatchString(name) || reserved {
			return nil, fmt.Errorf("client %q must have a portable lowercase name of 1–64 letters, digits or hyphens, starting with a letter", name)
		}
		if clients[name].Selection == nil {
			return nil, fmt.Errorf("client %q requires a selection", name)
		}
		selection, err := clients[name].Selection.Canonical()
		if err != nil {
			return nil, fmt.Errorf("client %q: %w", name, err)
		}
		result[name] = Client{Selection: selection}
	}
	return result, nil
}

func sortedClientNames(clients map[string]Client) []string {
	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
