package unicode17

import "sort"

type classEntry struct {
	lo, hi rune
	value  uint8
}
type decompositionEntry struct {
	point         rune
	offset, count int
}
type compositionEntry struct {
	pair  uint64
	point rune
}

func CombiningClass(value rune) uint8 {
	index := sort.Search(len(canonicalClasses), func(index int) bool { return canonicalClasses[index].hi >= value })
	if index < len(canonicalClasses) && canonicalClasses[index].lo <= value {
		return canonicalClasses[index].value
	}
	return 0
}

func appendDecomposed(destination []rune, value rune) []rune {
	const sBase, lBase, vBase, tBase = 0xac00, 0x1100, 0x1161, 0x11a7
	const lCount, vCount, tCount = 19, 21, 28
	offset := value - sBase
	if offset >= 0 && offset < lCount*vCount*tCount {
		destination = append(destination, lBase+offset/(vCount*tCount), vBase+(offset/tCount)%vCount)
		if tail := offset % tCount; tail != 0 {
			destination = append(destination, tBase+tail)
		}
		return destination
	}
	index := sort.Search(len(decompositions), func(index int) bool { return decompositions[index].point >= value })
	if index == len(decompositions) || decompositions[index].point != value {
		return append(destination, value)
	}
	entry := decompositions[index]
	// Recursion follows the finite, hash-verified Unicode decomposition graph.
	for _, part := range decompositionRunes[entry.offset : entry.offset+entry.count] {
		destination = appendDecomposed(destination, part)
	}
	return destination
}

func compose(left, right rune) (rune, bool) {
	const sBase, lBase, vBase, tBase = 0xac00, 0x1100, 0x1161, 0x11a7
	const lCount, vCount, tCount = 19, 21, 28
	if left >= lBase && left < lBase+lCount && right >= vBase && right < vBase+vCount {
		return sBase + ((left-lBase)*vCount+right-vBase)*tCount, true
	}
	if left >= sBase && left < sBase+lCount*vCount*tCount && (left-sBase)%tCount == 0 && right > tBase && right < tBase+tCount {
		return left + right - tBase, true
	}
	pair := uint64(left)<<21 | uint64(right)
	index := sort.Search(len(compositions), func(index int) bool { return compositions[index].pair >= pair })
	if index < len(compositions) && compositions[index].pair == pair {
		return compositions[index].point, true
	}
	return 0, false
}

// NFC implements canonical decomposition, stable ordering and composition.
func NFC(value string) string {
	runes := make([]rune, 0, len(value))
	for _, point := range value {
		runes = appendDecomposed(runes, point)
	}
	// Sort only combining runs; starters delimit canonical ordering segments.
	for start := 0; start < len(runes); {
		if CombiningClass(runes[start]) == 0 {
			start++
			continue
		}
		end := start + 1
		for end < len(runes) && CombiningClass(runes[end]) != 0 {
			end++
		}
		run := runes[start:end]
		sort.SliceStable(run, func(i, j int) bool { return CombiningClass(run[i]) < CombiningClass(run[j]) })
		start = end
	}
	if len(runes) == 0 {
		return ""
	}
	result := runes[:1]
	starter := 0
	previous := CombiningClass(runes[0])
	for _, point := range runes[1:] {
		class := CombiningClass(point)
		if combined, ok := compose(result[starter], point); ok && (previous == 0 || previous < class) {
			result[starter] = combined
			continue
		}
		result = append(result, point)
		if class == 0 {
			starter = len(result) - 1
		}
		previous = class
	}
	return string(result)
}
