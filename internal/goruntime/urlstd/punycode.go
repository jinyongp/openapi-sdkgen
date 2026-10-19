package urlstd

import "strings"

// RFC 3492 Bootstring, with scalar and arithmetic bounds checked before updates.
func adapt(delta uint64, count uint64, first bool) uint64 {
	if first {
		delta /= 700
	} else {
		delta /= 2
	}
	delta += delta / count
	var k uint64
	for delta > 455 {
		delta /= 35
		k += 36
	}
	return k + 36*delta/(delta+38)
}
func digit(value uint64) byte {
	if value < 26 {
		return byte(value) + 'a'
	}
	return byte(value-26) + '0'
}
func undigit(value byte) (uint64, bool) {
	switch {
	case value >= 'a' && value <= 'z':
		return uint64(value - 'a'), true
	case value >= 'A' && value <= 'Z':
		return uint64(value - 'A'), true
	case value >= '0' && value <= '9':
		return uint64(value - '0' + 26), true
	}
	return 0, false
}
func threshold(k, bias uint64) uint64 {
	if k <= bias+1 {
		return 1
	}
	if k >= bias+26 {
		return 26
	}
	return k - bias
}
func encodePunycode(value string) (string, error) {
	points := []rune(value)
	var out strings.Builder
	for _, r := range points {
		if r < 128 {
			out.WriteByte(byte(r))
		}
	}
	basic := uint64(out.Len())
	handled := basic
	if basic > 0 {
		out.WriteByte('-')
	}
	n, delta, bias := uint64(128), uint64(0), uint64(72)
	for handled < uint64(len(points)) {
		m := uint64(0x110000)
		for _, r := range points {
			if uint64(r) >= n && uint64(r) < m {
				m = uint64(r)
			}
		}
		if m == 0x110000 || m-n > (^uint64(0)-delta)/(handled+1) {
			return "", errURL
		}
		delta += (m - n) * (handled + 1)
		n = m
		for _, r := range points {
			if uint64(r) < n {
				if delta == ^uint64(0) {
					return "", errURL
				}
				delta++
			}
			if uint64(r) != n {
				continue
			}
			q := delta
			for k := uint64(36); ; k += 36 {
				t := threshold(k, bias)
				if q < t {
					break
				}
				out.WriteByte(digit(t + (q-t)%(36-t)))
				q = (q - t) / (36 - t)
			}
			out.WriteByte(digit(q))
			bias = adapt(delta, handled+1, handled == basic)
			delta = 0
			handled++
		}
		delta++
		n++
	}
	return out.String(), nil
}
func decodePunycode(value string) (string, error) {
	var out []rune
	position := 0
	if split := strings.LastIndexByte(value, '-'); split >= 0 {
		for i := 0; i < split; i++ {
			if value[i] >= 128 {
				return "", errURL
			}
			out = append(out, rune(value[i]))
		}
		position = split + 1
	}
	n, index, bias := uint64(128), uint64(0), uint64(72)
	for position < len(value) {
		old, weight := index, uint64(1)
		for k := uint64(36); ; k += 36 {
			if position == len(value) {
				return "", errURL
			}
			d, ok := undigit(value[position])
			position++
			if !ok || d > (^uint64(0)-index)/weight {
				return "", errURL
			}
			index += d * weight
			t := threshold(k, bias)
			if d < t {
				break
			}
			if weight > ^uint64(0)/(36-t) {
				return "", errURL
			}
			weight *= 36 - t
		}
		count := uint64(len(out) + 1)
		bias = adapt(index-old, count, old == 0)
		if index/count > 0x10ffff-n {
			return "", errURL
		}
		n += index / count
		index %= count
		if n >= 0xd800 && n <= 0xdfff {
			return "", errURL
		}
		out = append(out, 0)
		copy(out[int(index)+1:], out[int(index):])
		out[int(index)] = rune(n)
		index++
	}
	return string(out), nil
}
