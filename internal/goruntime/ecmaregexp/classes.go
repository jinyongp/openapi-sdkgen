package ecmaregexp

import (
	"openapi-sdkgen/internal/goruntime/unicode17"
	"sort"
	"strings"
)

func mergeRanges(ranges []Range) []Range {
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Lo == ranges[j].Lo {
			return ranges[i].Hi < ranges[j].Hi
		}
		return ranges[i].Lo < ranges[j].Lo
	})
	out := ranges[:0]
	for _, r := range ranges {
		if len(out) > 0 && r.Lo <= out[len(out)-1].Hi+1 {
			out[len(out)-1].Hi = max(out[len(out)-1].Hi, r.Hi)
		} else {
			out = append(out, r)
		}
	}
	return out
}
func complement(ranges []Range) []Range {
	ranges = mergeRanges(ranges)
	var out []Range
	next := rune(0)
	for _, r := range ranges {
		if next < r.Lo {
			out = append(out, Range{Lo: next, Hi: r.Lo - 1})
		}
		next = r.Hi + 1
	}
	if next <= 0x10ffff {
		out = append(out, Range{Lo: next, Hi: 0x10ffff})
	}
	return out
}
func rangeContains(ranges []Range, value rune) bool {
	index := sort.Search(len(ranges), func(index int) bool { return ranges[index].Hi >= value })
	return index < len(ranges) && ranges[index].Lo <= value
}
func foldedRanges(ranges []Range) []Range {
	ranges = mergeRanges(ranges)
	var additions []Range
	unicode17.VisitSimpleFolds(func(from, to rune) {
		if rangeContains(ranges, from) {
			additions = append(additions, Range{Lo: to, Hi: to})
		}
	})
	return mergeRanges(append(ranges, additions...))
}
func (p *compiler) classKey(start int, flags Flags) string {
	bytes := make([]byte, 1+4*(p.position-start))
	bytes[0] = byte(flags)
	for i, r := range p.text[start:p.position] {
		for j := 0; j < 4; j++ {
			bytes[1+4*i+j] = byte(uint32(r) >> (8 * j))
		}
	}
	return string(bytes)
}
func (p *compiler) classNode(ranges []Range, flags Flags, negative bool, key string) int {
	if index, ok := p.classCache[key]; ok {
		return index
	}
	ranges = mergeRanges(ranges)
	if flags&IgnoreCase != 0 {
		ranges = foldedRanges(ranges)
	}
	if negative {
		ranges = complement(ranges)
	}
	offset := len(p.program.Ranges)
	p.program.Ranges = append(p.program.Ranges, ranges...)
	index := p.node(Instruction{Op: CharacterClass, A: offset, B: len(ranges), Flags: flags})
	p.classCache[key] = index
	return index
}
func (p *compiler) characterClass(flags Flags) (int, error) {
	start := p.position - 1
	negative := false
	if p.position < len(p.text) && p.text[p.position] == '^' {
		negative = true
		p.position++
	}
	var ranges []Range
	seen := make(map[Range]bool)
	add := func(values []Range) {
		for _, r := range values {
			if !seen[r] {
				seen[r] = true
				ranges = append(ranges, r)
			}
		}
	}
	for p.position < len(p.text) && p.text[p.position] != ']' {
		if err := p.step(); err != nil {
			return 0, err
		}
		first, single, err := p.classTerm(flags)
		if err != nil {
			return 0, err
		}
		if p.position+1 < len(p.text) && p.text[p.position] == '-' && p.text[p.position+1] != ']' {
			p.position++
			last, lastSingle, err := p.classTerm(flags)
			if err != nil {
				return 0, err
			}
			if !single || !lastSingle || first[0].Lo > last[0].Lo {
				return 0, p.invalid("invalid character class range")
			}
			add([]Range{{Lo: first[0].Lo, Hi: last[0].Lo}})
		} else {
			add(first)
		}
	}
	if p.position == len(p.text) {
		return 0, p.invalid("unterminated character class")
	}
	p.position++
	return p.classNode(ranges, flags, negative, p.classKey(start, flags)), nil
}
func (p *compiler) classTerm(flags Flags) ([]Range, bool, error) {
	if p.position == len(p.text) {
		return nil, false, p.invalid("unterminated character class")
	}
	r := p.text[p.position]
	p.position++
	if r == '\\' {
		return p.escapeCharacter(true, flags)
	}
	return []Range{{Lo: r, Hi: r}}, true, nil
}
func (p *compiler) escapeAtom(flags Flags) (int, error) {
	start := p.position - 1
	if p.position == len(p.text) {
		return 0, p.invalid("trailing escape")
	}
	r := p.text[p.position]
	if r == 'b' || r == 'B' {
		p.position++
		return p.node(Instruction{Op: WordBoundary, Flags: flags, Negative: r == 'B'}), nil
	}
	if r >= '1' && r <= '9' {
		text, err := p.decimal()
		if err != nil {
			return 0, err
		}
		number := decimalValue(text)
		group := int(number)
		if number > uint64(^uint(0)>>1) {
			group = int(^uint(0) >> 1)
		}
		node := p.node(Instruction{Op: Reference, Group: group, Flags: flags})
		p.refs = append(p.refs, pendingReference{node: node})
		return node, nil
	}
	if r == 'k' {
		p.position++
		if p.position == len(p.text) || p.text[p.position] != '<' {
			return 0, p.invalid("invalid named reference")
		}
		p.position++
		name, err := p.groupName()
		if err != nil {
			return 0, err
		}
		node := p.node(Instruction{Op: Reference, Flags: flags})
		p.refs = append(p.refs, pendingReference{node: node, name: name})
		return node, nil
	}
	ranges, single, err := p.escapeCharacter(false, flags)
	if err != nil {
		return 0, err
	}
	if single {
		return p.node(Instruction{Op: Literal, Value: ranges[0].Lo, Flags: flags}), nil
	}
	return p.classNode(ranges, flags, false, p.classKey(start, flags)), nil
}
func (p *compiler) escapeCharacter(inClass bool, flags Flags) ([]Range, bool, error) {
	if p.position == len(p.text) {
		return nil, false, p.invalid("trailing escape")
	}
	r := p.text[p.position]
	p.position++
	single := func(r rune) ([]Range, bool, error) { return []Range{{Lo: r, Hi: r}}, true, nil }
	switch r {
	case 'd', 'D':
		ranges := []Range{{Lo: '0', Hi: '9'}}
		if r == 'D' {
			ranges = complement(ranges)
		}
		return ranges, false, nil
	case 'w', 'W':
		ranges := []Range{{Lo: '0', Hi: '9'}, {Lo: 'A', Hi: 'Z'}, {Lo: '_', Hi: '_'}, {Lo: 'a', Hi: 'z'}}
		if r == 'W' {
			if flags&IgnoreCase != 0 {
				base := ranges
				unicode17.VisitSimpleFolds(func(from, to rune) {
					if rangeContains(base, to) {
						ranges = append(ranges, Range{Lo: from, Hi: from})
					}
				})
			}
			ranges = complement(ranges)
		}
		return ranges, false, nil
	case 's', 'S':
		ranges := []Range{{Lo: 0x9, Hi: 0xd}, {Lo: 0x20, Hi: 0x20}, {Lo: 0xa0, Hi: 0xa0}, {Lo: 0x1680, Hi: 0x1680}, {Lo: 0x2000, Hi: 0x200a}, {Lo: 0x2028, Hi: 0x2029}, {Lo: 0x202f, Hi: 0x202f}, {Lo: 0x205f, Hi: 0x205f}, {Lo: 0x3000, Hi: 0x3000}, {Lo: 0xfeff, Hi: 0xfeff}}
		if r == 'S' {
			ranges = complement(ranges)
		}
		return ranges, false, nil
	case 'p', 'P':
		if p.position == len(p.text) || p.text[p.position] != '{' {
			return nil, false, p.invalid("invalid Unicode property escape")
		}
		p.position++
		start := p.position
		for p.position < len(p.text) && p.text[p.position] != '}' {
			if err := p.step(); err != nil {
				return nil, false, err
			}
			p.position++
		}
		if p.position == len(p.text) {
			return nil, false, p.invalid("unterminated Unicode property escape")
		}
		text := string(p.text[start:p.position])
		p.position++
		name, value := "", text
		if left, right, found := strings.Cut(text, "="); found {
			if left == "" || right == "" {
				return nil, false, p.invalid("invalid Unicode property expression")
			}
			name, value = left, right
		}
		property, ok := unicode17.LookupProperty(name, value)
		if !ok {
			return nil, false, p.invalid("invalid Unicode property")
		}
		ranges := property.AppendRanges(nil)
		if r == 'P' {
			ranges = complement(ranges)
		}
		return ranges, false, nil
	case 't':
		return single('\t')
	case 'n':
		return single('\n')
	case 'v':
		return single('\v')
	case 'f':
		return single('\f')
	case 'r':
		return single('\r')
	case 'b':
		if inClass {
			return single('\b')
		}
	case '0':
		if p.position < len(p.text) && p.text[p.position] >= '0' && p.text[p.position] <= '9' {
			return nil, false, p.invalid("legacy octal escape in Unicode pattern")
		}
		return single(0)
	case 'c':
		if p.position == len(p.text) {
			return nil, false, p.invalid("incomplete control escape")
		}
		c := p.text[p.position]
		p.position++
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			return single(c & 31)
		}
		return nil, false, p.invalid("invalid control escape")
	case 'x':
		value, err := p.hex(2)
		if err != nil {
			return nil, false, err
		}
		return single(value)
	case 'u':
		value, err := p.unicodeEscape()
		if err != nil {
			return nil, false, err
		}
		return single(value)
	}
	if strings.ContainsRune("^$\\.*+?()[]{}|/", r) || inClass && r == '-' {
		return single(r)
	}
	return nil, false, p.invalid("invalid identity escape in Unicode pattern")
}
func hexDigit(r rune) (rune, bool) {
	switch {
	case r >= '0' && r <= '9':
		return r - '0', true
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10, true
	case r >= 'A' && r <= 'F':
		return r - 'A' + 10, true
	}
	return 0, false
}
func (p *compiler) hex(count int) (rune, error) {
	if p.position+count > len(p.text) {
		return 0, p.invalid("incomplete hexadecimal escape")
	}
	var result rune
	for range count {
		value, ok := hexDigit(p.text[p.position])
		if !ok {
			return 0, p.invalid("invalid hexadecimal escape")
		}
		p.position++
		result = result*16 + value
	}
	return result, nil
}
func (p *compiler) unicodeEscape() (rune, error) {
	if p.position < len(p.text) && p.text[p.position] == '{' {
		p.position++
		start := p.position
		var value rune
		for p.position < len(p.text) && p.text[p.position] != '}' {
			if err := p.step(); err != nil {
				return 0, err
			}
			n, ok := hexDigit(p.text[p.position])
			if !ok || value > 0x10ffff/16 {
				return 0, p.invalid("invalid Unicode code point escape")
			}
			value = value*16 + n
			if value > 0x10ffff {
				return 0, p.invalid("Unicode escape exceeds code point range")
			}
			p.position++
		}
		if start == p.position || p.position == len(p.text) {
			return 0, p.invalid("unterminated Unicode code point escape")
		}
		p.position++
		return value, nil
	}
	value, err := p.hex(4)
	if err != nil {
		return 0, err
	}
	if value >= 0xd800 && value <= 0xdbff && p.position+6 <= len(p.text) && p.text[p.position] == '\\' && p.text[p.position+1] == 'u' && p.text[p.position+2] != '{' {
		old := p.position
		p.position += 2
		tail, err := p.hex(4)
		if err == nil && tail >= 0xdc00 && tail <= 0xdfff {
			return 0x10000 + (value-0xd800)*0x400 + tail - 0xdc00, nil
		}
		p.position = old
	}
	return value, nil
}
