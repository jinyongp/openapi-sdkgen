package unicode17

import "sort"

type IDNAStatus uint8

const (
	IDNAValid IDNAStatus = iota
	IDNAIgnored
	IDNAMapped
	IDNADeviation
	IDNADisallowed
	IDNASTD3Valid
	IDNASTD3Mapped
)

type idnaEntry struct {
	lo, hi  rune
	status  IDNAStatus
	mapping string
}
type joiningEntry struct {
	lo, hi rune
	value  string
}
type bidiEntry struct {
	lo, hi rune
	value  string
}

// LookupIDNA returns raw UTS #46 mapping data; the URL parser owns its policy.
func LookupIDNA(value rune) (IDNAStatus, string) {
	index := sort.Search(len(idnaMappings), func(index int) bool { return idnaMappings[index].hi >= value })
	if index == len(idnaMappings) || idnaMappings[index].lo > value {
		return IDNADisallowed, ""
	}
	entry := idnaMappings[index]
	return entry.status, entry.mapping
}
func JoiningType(value rune) string {
	index := sort.Search(len(joiningTypes), func(index int) bool { return joiningTypes[index].hi >= value })
	if index < len(joiningTypes) && joiningTypes[index].lo <= value {
		return joiningTypes[index].value
	}
	return "U"
}
func BidiClass(value rune) string {
	index := sort.Search(len(bidiClasses), func(index int) bool { return bidiClasses[index].hi >= value })
	if index < len(bidiClasses) && bidiClasses[index].lo <= value {
		return bidiClasses[index].value
	}
	return "L"
}
