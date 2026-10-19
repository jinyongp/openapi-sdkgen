package urlstd

import "strings"

type encodeSet uint8

const (
	encodeC0 encodeSet = iota
	encodeFragment
	encodeQuery
	encodeSpecialQuery
	encodePath
	encodeUserinfo
)

func escapeByte(c byte, set encodeSet) bool {
	if c <= 31 || c >= 127 {
		return true
	}
	switch set {
	case encodeFragment:
		return c == ' ' || c == '"' || c == '<' || c == '>' || c == '`'
	case encodeQuery, encodeSpecialQuery:
		return c == ' ' || c == '"' || c == '#' || c == '<' || c == '>' || set == encodeSpecialQuery && c == '\''
	case encodePath, encodeUserinfo:
		if c == ' ' || c == '"' || c == '#' || c == '<' || c == '>' || c == '?' || c == '`' || c == '{' || c == '}' || c == '^' {
			return true
		}
		return set == encodeUserinfo && strings.ContainsRune("/:;=@[\\]^|", rune(c))
	}
	return false
}
func percent(value string, set encodeSet) string {
	at := 0
	for at < len(value) && !escapeByte(value[at], set) {
		at++
	}
	if at == len(value) {
		return value
	}
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	out.Grow(len(value) + 8)
	out.WriteString(value[:at])
	for ; at < len(value); at++ {
		c := value[at]
		if escapeByte(c, set) {
			out.WriteByte('%')
			out.WriteByte(hex[c>>4])
			out.WriteByte(hex[c&15])
		} else {
			out.WriteByte(c)
		}
	}
	return out.String()
}
func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
func percentDecode(value string) string {
	if !strings.ContainsRune(value, '%') {
		return value
	}
	out := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '%' && i+2 < len(value) {
			a, ok := unhex(value[i+1])
			b, ok2 := unhex(value[i+2])
			if ok && ok2 {
				out = append(out, a*16+b)
				i += 2
				continue
			}
		}
		out = append(out, value[i])
	}
	return string(out)
}
