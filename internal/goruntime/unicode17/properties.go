// Package unicode17 provides fixed Unicode data for the generated Go runtime.
// It does not use the Go toolchain's independently versioned Unicode tables.
package unicode17

import "sort"

const Version = "17.0.0"

type Range struct{ Lo, Hi rune }
type propertyEntry struct{ offset, count int }
type propertyAlias struct {
	name  string
	index int
}

// Property is an immutable view of a Unicode character set.
type Property struct{ index int }

func (property Property) Contains(value rune) bool {
	if property.index <= 0 || property.index > len(propertyEntries) {
		return false
	}
	entry := propertyEntries[property.index-1]
	ranges := propertyRanges[entry.offset : entry.offset+entry.count]
	index := sort.Search(len(ranges), func(index int) bool { return ranges[index].Hi >= value })
	return index < len(ranges) && ranges[index].Lo <= value
}

// AppendRanges copies ranges so a compiler cannot mutate shared Unicode data.
func (property Property) AppendRanges(destination []Range) []Range {
	if property.index <= 0 || property.index > len(propertyEntries) {
		return destination
	}
	entry := propertyEntries[property.index-1]
	return append(destination, propertyRanges[entry.offset:entry.offset+entry.count]...)
}

// LookupProperty implements ECMAScript's exact, case-sensitive property aliases.
// An empty name denotes a lone binary property or General_Category value.
func LookupProperty(name, value string) (Property, bool) {
	key := value
	switch name {
	case "":
	case "General_Category", "gc":
		key = "gc=" + value
	case "Script", "sc":
		key = "sc=" + value
	case "Script_Extensions", "scx":
		key = "scx=" + value
	default:
		return Property{}, false
	}
	lookup := func(key string) (Property, bool) {
		index := sort.Search(len(propertyAliases), func(index int) bool { return propertyAliases[index].name >= key })
		if index == len(propertyAliases) || propertyAliases[index].name != key {
			return Property{}, false
		}
		return Property{index: propertyAliases[index].index + 1}, true
	}
	property, ok := lookup(key)
	if !ok && name == "" {
		return lookup("gc=" + value)
	}
	return property, ok
}
