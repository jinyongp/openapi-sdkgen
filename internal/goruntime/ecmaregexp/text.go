package ecmaregexp

import "unicode/utf8"

func rawForward(value string, offset int) (rune, int) {
	if value[offset] < utf8.RuneSelf {
		return rune(value[offset]), offset + 1
	}
	r, size := utf8.DecodeRuneInString(value[offset:])
	if size == 1 && offset+2 < len(value) && value[offset] == 0xed && value[offset+1] >= 0xa0 && value[offset+1] <= 0xbf && value[offset+2] >= 0x80 && value[offset+2] <= 0xbf {
		r = rune(value[offset]&15)<<12 | rune(value[offset+1]&63)<<6 | rune(value[offset+2]&63)
		size = 3
	}
	return r, offset + size
}
func rawBackward(value string, offset int) (rune, int) {
	if value[offset-1] < utf8.RuneSelf {
		return rune(value[offset-1]), offset - 1
	}
	if offset >= 3 && value[offset-3] == 0xed && value[offset-2] >= 0xa0 && value[offset-2] <= 0xbf && value[offset-1] >= 0x80 && value[offset-1] <= 0xbf {
		r := rune(value[offset-3]&15)<<12 | rune(value[offset-2]&63)<<6 | rune(value[offset-1]&63)
		return r, offset - 3
	}
	r, size := utf8.DecodeLastRuneInString(value[:offset])
	return r, offset - size
}
func forward(value string, offset int) (rune, int, bool) {
	if offset >= len(value) {
		return 0, offset, false
	}
	r, next := rawForward(value, offset)
	if r >= 0xd800 && r <= 0xdbff && next < len(value) {
		tail, end := rawForward(value, next)
		if tail >= 0xdc00 && tail <= 0xdfff {
			return 0x10000 + (r-0xd800)*0x400 + tail - 0xdc00, end, true
		}
	}
	return r, next, true
}
func backward(value string, offset int) (rune, int, bool) {
	if offset <= 0 {
		return 0, offset, false
	}
	r, next := rawBackward(value, offset)
	if r >= 0xdc00 && r <= 0xdfff && next > 0 {
		lead, start := rawBackward(value, next)
		if lead >= 0xd800 && lead <= 0xdbff {
			return 0x10000 + (lead-0xd800)*0x400 + r - 0xdc00, start, true
		}
	}
	return r, next, true
}
func readPoint(value string, offset, direction int) (rune, int, bool) {
	if direction < 0 {
		return backward(value, offset)
	}
	return forward(value, offset)
}

// codePoints preserves the wire layer's WTF-8 lone surrogates and combines pairs.
func codePoints(value string) []rune {
	result := make([]rune, 0, len(value))
	for offset := 0; offset < len(value); {
		r, next, _ := forward(value, offset)
		result = append(result, r)
		offset = next
	}
	return result
}
