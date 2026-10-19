package ecmaregexp

import "unicode/utf8"

// codePoints preserves WTF-8 lone surrogates from the lossless JSON wire layer.
// Adjacent UTF-16 surrogate units combine under the Unicode matching mode.
func codePoints(value string) []rune {
	result := make([]rune, 0, len(value))
	for offset := 0; offset < len(value); {
		r, size := utf8.DecodeRuneInString(value[offset:])
		if size == 1 && offset+2 < len(value) && value[offset] == 0xed && value[offset+1] >= 0xa0 && value[offset+1] <= 0xbf && value[offset+2] >= 0x80 && value[offset+2] <= 0xbf {
			r = rune(value[offset]&15)<<12 | rune(value[offset+1]&63)<<6 | rune(value[offset+2]&63)
			size = 3
		}
		offset += size
		if r >= 0xdc00 && r <= 0xdfff && len(result) > 0 && result[len(result)-1] >= 0xd800 && result[len(result)-1] <= 0xdbff {
			result[len(result)-1] = 0x10000 + (result[len(result)-1]-0xd800)*0x400 + r - 0xdc00
			continue
		}
		result = append(result, r)
	}
	return result
}
