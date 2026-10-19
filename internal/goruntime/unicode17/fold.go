package unicode17

import "sort"

type foldEntry struct{ from, to rune }

// Fold returns the Unicode simple case fold used by ECMAScript Unicode matching.
func Fold(value rune) rune {
	index := sort.Search(len(caseFolds), func(index int) bool { return caseFolds[index].from >= value })
	if index < len(caseFolds) && caseFolds[index].from == value {
		return caseFolds[index].to
	}
	return value
}

// FoldEquivalents copies all code points sharing value's simple case fold.
func FoldEquivalents(value rune, destination []rune) []rune {
	folded := Fold(value)
	destination = append(destination, folded)
	for _, entry := range caseFolds {
		if entry.to == folded {
			destination = append(destination, entry.from)
		}
	}
	return destination
}
