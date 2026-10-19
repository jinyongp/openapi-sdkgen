package ecmaregexp

import "sort"

func rangeContains(ranges []Range, value rune) bool {
	index := sort.Search(len(ranges), func(index int) bool { return ranges[index].Hi >= value })
	return index < len(ranges) && ranges[index].Lo <= value
}
