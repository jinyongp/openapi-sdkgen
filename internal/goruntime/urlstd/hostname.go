package urlstd

import (
	"openapi-sdkgen/internal/goruntime/unicode17"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HostnameToASCII applies DNS label limits and IDNA2008 contextual rules after
// UTS 46 mapping. URL host parsing intentionally uses its own non-strict policy.
func HostnameToASCII(value string) (string, error) {
	if value == "" || !utf8.ValidString(value) {
		return "", errURL
	}
	mapped, err := DomainToASCII(value)
	if err != nil || len(mapped) > 253 {
		return "", errURL
	}
	labels := strings.Split(mapped, ".")
	decoded := make([]string, len(labels))
	bidi := false
	for index, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errURL
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return "", errURL
			}
		}
		text := label
		if strings.HasPrefix(label, "xn--") {
			text, err = decodePunycode(label[4:])
			if err != nil || text == "" || ascii(text) {
				return "", errURL
			}
			canonical, err := DomainToASCII(text)
			if err != nil || canonical != label {
				return "", errURL
			}
		}
		if !validHostnameLabel(text) || !validJoiners(text) {
			return "", errURL
		}
		decoded[index] = text
		for _, point := range text {
			class := unicode17.BidiClass(point)
			bidi = bidi || class == "R" || class == "AL" || class == "AN"
		}
	}
	if bidi {
		for _, label := range decoded {
			if !validBidi(label) {
				return "", errURL
			}
		}
	}
	return mapped, nil
}
func validHostnameLabel(label string) bool {
	points := []rune(label)
	if len(points) == 0 || points[0] == '-' || points[len(points)-1] == '-' || unicode17.IsMark(points[0]) || len(points) >= 4 && points[2] == '-' && points[3] == '-' {
		return false
	}
	arabic, extended := false, false
	japanese := false
	for _, point := range points {
		arabic = arabic || point >= 0x660 && point <= 0x669
		extended = extended || point >= 0x6f0 && point <= 0x6f9
		japanese = japanese || unicode.In(point, unicode.Hiragana, unicode.Katakana, unicode.Han)
	}
	if arabic && extended {
		return false
	}
	for index, point := range points {
		if point == 0x640 || point == 0x7fa || point == 0x302e || point == 0x302f || point >= 0x3031 && point <= 0x3035 || point == 0x303b || point >= 0x1100 && point <= 0x11ff {
			return false
		}
		special := strings.ContainsRune("-\u00b7\u0375\u05f3\u05f4\u30fb\u200c\u200d\u0f0b\u3007\u06fd\u06fe", point)
		if !unicode.IsLetter(point) && !unicode.IsMark(point) && !unicode.Is(unicode.Nd, point) && !special {
			return false
		}
		switch point {
		case 0xb7:
			if index == 0 || index+1 == len(points) || points[index-1] != 'l' || points[index+1] != 'l' {
				return false
			}
		case 0x375:
			if index+1 == len(points) || !unicode.In(points[index+1], unicode.Greek) {
				return false
			}
		case 0x5f3, 0x5f4:
			if index == 0 || !unicode.In(points[index-1], unicode.Hebrew) {
				return false
			}
		case 0x30fb:
			if !japanese {
				return false
			}
		}
	}
	return true
}
