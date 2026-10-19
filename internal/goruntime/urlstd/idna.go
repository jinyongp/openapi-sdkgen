package urlstd

import (
	"openapi-sdkgen/internal/goruntime/unicode17"
	"strings"
	"unicode/utf8"
)

func ascii(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 128 {
			return false
		}
	}
	return true
}
func lower(value string) string {
	for i := 0; i < len(value); i++ {
		if value[i] >= 'A' && value[i] <= 'Z' {
			data := []byte(value)
			for j := i; j < len(data); j++ {
				if data[j] >= 'A' && data[j] <= 'Z' {
					data[j] += 32
				}
			}
			return string(data)
		}
	}
	return value
}

// DomainToASCII implements the pinned WHATWG non-strict domain parser. ASCII
// input bypasses UTS46 validity by design; host parsing separately rejects URL
// delimiters. Non-ASCII uses nontransitional mapping, NFC, Bidi and ContextJ.
func DomainToASCII(value string) (string, error) {
	if ascii(value) {
		return lower(value), nil
	}
	var mapped strings.Builder
	mapped.Grow(min(len(value), 256))
	for _, point := range value {
		status, replacement := unicode17.LookupIDNA(point)
		switch status {
		case unicode17.IDNAIgnored:
		case unicode17.IDNAMapped, unicode17.IDNASTD3Mapped:
			mapped.WriteString(replacement)
		default:
			mapped.WriteRune(point)
		}
	}
	labels := strings.Split(unicode17.NFC(mapped.String()), ".")
	bidi := false
	for i, label := range labels {
		if strings.HasPrefix(label, "xn--") {
			if !ascii(label) {
				return "", errURL
			}
			decoded, err := decodePunycode(label[4:])
			if err != nil || decoded == "" || ascii(decoded) {
				return "", errURL
			}
			labels[i] = decoded
		}
		for _, point := range labels[i] {
			class := unicode17.BidiClass(point)
			if class == "R" || class == "AL" || class == "AN" {
				bidi = true
			}
		}
	}
	for i, label := range labels {
		if label == "" {
			continue
		}
		if unicode17.NFC(label) != label || strings.HasPrefix(label, "xn--") || strings.ContainsRune(label, '.') {
			return "", errURL
		}
		first, _ := utf8.DecodeRuneInString(label)
		if unicode17.IsMark(first) {
			return "", errURL
		}
		for _, point := range label {
			status, _ := unicode17.LookupIDNA(point)
			if status != unicode17.IDNAValid && status != unicode17.IDNADeviation && status != unicode17.IDNASTD3Valid {
				return "", errURL
			}
		}
		if !validJoiners(label) || bidi && !validBidi(label) {
			return "", errURL
		}
		if !ascii(label) {
			encoded, err := encodePunycode(label)
			if err != nil {
				return "", err
			}
			labels[i] = "xn--" + encoded
		}
	}
	return strings.Join(labels, "."), nil
}
func validJoiners(label string) bool {
	points := []rune(label)
	for i, point := range points {
		if point != 0x200c && point != 0x200d {
			continue
		}
		if i > 0 && unicode17.CombiningClass(points[i-1]) == 9 {
			continue
		}
		if point == 0x200d {
			return false
		}
		left := i - 1
		for left >= 0 && unicode17.JoiningType(points[left]) == "T" {
			left--
		}
		right := i + 1
		for right < len(points) && unicode17.JoiningType(points[right]) == "T" {
			right++
		}
		if left < 0 || right == len(points) {
			return false
		}
		a, b := unicode17.JoiningType(points[left]), unicode17.JoiningType(points[right])
		if (a != "L" && a != "D") || (b != "R" && b != "D") {
			return false
		}
	}
	return true
}
func validBidi(label string) bool {
	first, _ := utf8.DecodeRuneInString(label)
	class := unicode17.BidiClass(first)
	rtl := class == "R" || class == "AL"
	if !rtl && class != "L" {
		return false
	}
	last := ""
	en, an := false, false
	for _, point := range label {
		c := unicode17.BidiClass(point)
		switch c {
		case "R", "AL", "AN":
			if !rtl {
				return false
			}
		case "L":
			if rtl {
				return false
			}
		case "EN", "ES", "CS", "ET", "ON", "BN", "NSM":
		default:
			return false
		}
		if c != "NSM" {
			last = c
		}
		en = en || c == "EN"
		an = an || c == "AN"
	}
	if rtl {
		return !(en && an) && (last == "R" || last == "AL" || last == "EN" || last == "AN")
	}
	return last == "L" || last == "EN"
}
