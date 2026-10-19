package urlstd

import (
	"net/netip"
	"strconv"
	"strings"
)

func forbiddenHost(point rune) bool {
	return point == 0 || strings.ContainsRune("\t\n\r #/:<>?@[\\]^|", point)
}
func parseHost(value string, opaque bool) (string, error) {
	if strings.HasPrefix(value, "[") {
		if !strings.HasSuffix(value, "]") {
			return "", errURL
		}
		address, err := netip.ParseAddr(value[1 : len(value)-1])
		if err != nil || !address.Is6() || address.Zone() != "" {
			return "", errURL
		}
		bytes := address.As16()
		var pieces [8]uint16
		for i := range pieces {
			pieces[i] = uint16(bytes[2*i])<<8 | uint16(bytes[2*i+1])
		}
		start, length := -1, 1
		for i := 0; i < len(pieces); {
			if pieces[i] != 0 {
				i++
				continue
			}
			end := i + 1
			for end < len(pieces) && pieces[end] == 0 {
				end++
			}
			if end-i > length {
				start, length = i, end-i
			}
			i = end
		}
		var out strings.Builder
		out.WriteByte('[')
		for i := 0; i < 8; {
			if i == start {
				out.WriteString("::")
				i += length
				continue
			}
			if i > 0 && i != start+length {
				out.WriteByte(':')
			}
			out.WriteString(strconv.FormatUint(uint64(pieces[i]), 16))
			i++
		}
		out.WriteByte(']')
		return out.String(), nil
	}
	if opaque {
		for _, point := range value {
			if forbiddenHost(point) {
				return "", errURL
			}
		}
		return percent(value, encodeC0), nil
	}
	if value == "" {
		return "", nil
	}
	domain := percentDecode(value)
	if !utf8Valid(domain) {
		return "", errURL
	}
	domain, err := DomainToASCII(domain)
	if err != nil || domain == "" {
		return "", errURL
	}
	for _, point := range domain {
		if forbiddenHost(point) || point <= 31 || point == 127 || point == '%' {
			return "", errURL
		}
	}
	if endsNumber(domain) {
		return ipv4(domain)
	}
	return domain, nil
}
func utf8Valid(value string) bool { return scalar(value) == value }
func ipv4Number(value string) (uint64, bool) {
	if value == "" {
		return 0, false
	}
	base := uint64(10)
	if len(value) >= 2 && value[0] == '0' && (value[1] == 'x' || value[1] == 'X') {
		base = 16
		value = value[2:]
	} else if len(value) >= 2 && value[0] == '0' {
		base = 8
		value = value[1:]
	}
	var result uint64
	for i := 0; i < len(value); i++ {
		d, ok := unhex(value[i])
		if !ok || uint64(d) >= base || result > (^uint64(0)-uint64(d))/base {
			return 0, false
		}
		result = result*base + uint64(d)
	}
	return result, true
}
func endsNumber(domain string) bool {
	parts := strings.Split(domain, ".")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	last := parts[len(parts)-1]
	if last == "" {
		return false
	}
	digits := true
	for i := 0; i < len(last); i++ {
		if last[i] < '0' || last[i] > '9' {
			digits = false
			break
		}
	}
	if digits {
		return true
	}
	if len(last) >= 2 && last[0] == '0' && (last[1] == 'x' || last[1] == 'X') {
		for i := 2; i < len(last); i++ {
			if _, ok := unhex(last[i]); !ok {
				return false
			}
		}
		return true
	}
	_, ok := ipv4Number(last)
	return ok
}
func ipv4(value string) (string, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 4 {
		return "", errURL
	}
	var number uint64
	for i, part := range parts {
		n, ok := ipv4Number(part)
		if !ok {
			return "", errURL
		}
		if i < len(parts)-1 {
			if n > 255 {
				return "", errURL
			}
			number += n << uint(8*(3-i))
		} else {
			limit := uint64(1) << uint(8*(5-len(parts)))
			if n >= limit {
				return "", errURL
			}
			number += n
		}
	}
	return strconv.FormatUint(number>>24, 10) + "." + strconv.FormatUint(number>>16&255, 10) + "." + strconv.FormatUint(number>>8&255, 10) + "." + strconv.FormatUint(number&255, 10), nil
}
