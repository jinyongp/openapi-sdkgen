package ecmaregexp

import (
	"context"
	"math"
	"sort"
	"strings"

	"openapi-sdkgen/internal/goruntime/unicode17"
)

type groupFrame struct {
	sequence, alternatives []int
	flags                  Flags
	capture, firstGroup    int
	negative, backward     bool
	look                   bool
	alt, branch, outerPath int
}
type branchPath struct{ parent, alt, branch, depth int }
type declaration struct{ group, path int }
type pendingReference struct {
	node int
	name string
}
type compiler struct {
	classCache map[string]int
	text       []rune
	position   int
	program    Program
	frames     []groupFrame
	paths      []branchPath
	names      map[string][]declaration
	refs       []pendingReference
	nextAlt    int
	work       uint64
	ctx        context.Context
}

func Compile(source string) (*Program, error) { return CompileContext(context.Background(), source) }
func CompileContext(ctx context.Context, source string) (*Program, error) {
	p := &compiler{classCache: make(map[string]int), text: codePoints(source), program: Program{Version: Version, Nodes: []Instruction{{Op: Empty}}}, names: make(map[string][]declaration), ctx: ctx, nextAlt: 1}
	p.frames = append(p.frames, groupFrame{firstGroup: 1, outerPath: -1})
	for p.position < len(p.text) {
		if err := p.step(); err != nil {
			return nil, err
		}
		switch p.text[p.position] {
		case '(':
			if err := p.openGroup(); err != nil {
				return nil, err
			}
		case ')':
			if len(p.frames) == 1 {
				return nil, p.invalid("unmatched closing parenthesis")
			}
			if err := p.closeGroup(); err != nil {
				return nil, err
			}
		case '|':
			p.position++
			p.nextAlternative()
		case '*', '+', '?', '{':
			if err := p.quantifier(); err != nil {
				return nil, err
			}
		default:
			node, err := p.atom()
			if err != nil {
				return nil, err
			}
			p.push(node)
		}
	}
	if len(p.frames) != 1 {
		return nil, p.invalid("unterminated group")
	}
	p.program.Root = p.finish(p.frames[0])
	if err := p.resolveNames(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &p.program, nil
}
func (p *compiler) step() error {
	p.work++
	if p.work > MaxSteps {
		return ErrComplexity
	}
	if p.work%1024 == 0 {
		return p.ctx.Err()
	}
	return nil
}
func (p *compiler) invalid(message string) error {
	return &SyntaxError{Offset: p.position, Message: message}
}
func (p *compiler) top() *groupFrame { return &p.frames[len(p.frames)-1] }
func (p *compiler) node(node Instruction) int {
	p.program.Nodes = append(p.program.Nodes, node)
	return len(p.program.Nodes) - 1
}
func (p *compiler) push(index int) { p.top().sequence = append(p.top().sequence, index) }
func (p *compiler) combine(op Op, nodes []int) int {
	if len(nodes) == 0 {
		return 0
	}
	if len(nodes) == 1 {
		return nodes[0]
	}
	node := Instruction{Op: op, A: len(p.program.Edges), B: len(nodes)}
	p.program.Edges = append(p.program.Edges, nodes...)
	for _, index := range nodes {
		child := p.program.Nodes[index]
		if child.FirstGroup != 0 && (node.FirstGroup == 0 || child.FirstGroup < node.FirstGroup) {
			node.FirstGroup = child.FirstGroup
		}
		node.LastGroup = max(node.LastGroup, child.LastGroup)
	}
	return p.node(node)
}
func (p *compiler) nextAlternative() {
	index := p.combine(Sequence, p.top().sequence)
	f := p.top()
	f.alternatives = append(f.alternatives, index)
	f.sequence = nil
	f.branch++
}
func (p *compiler) finish(frame groupFrame) int {
	child := p.combine(Sequence, frame.sequence)
	if len(frame.alternatives) > 0 {
		frame.alternatives = append(frame.alternatives, child)
		child = p.combine(Alternative, frame.alternatives)
	}
	return child
}
func (p *compiler) currentPath() int {
	frame := p.top()
	depth := 1
	if frame.outerPath >= 0 {
		depth = p.paths[frame.outerPath].depth + 1
	}
	p.paths = append(p.paths, branchPath{parent: frame.outerPath, alt: frame.alt, branch: frame.branch, depth: depth})
	return len(p.paths) - 1
}
func (p *compiler) openGroup() error {
	p.position++
	if len(p.frames) > MaxNesting {
		return ErrComplexity
	}
	parent := *p.top()
	frame := groupFrame{flags: parent.flags, firstGroup: p.program.Groups + 1, outerPath: p.currentPath(), alt: p.nextAlt}
	p.nextAlt++
	name := ""
	capture := true
	if p.position < len(p.text) && p.text[p.position] == '?' {
		p.position++
		if p.position == len(p.text) {
			return p.invalid("incomplete group")
		}
		switch p.text[p.position] {
		case ':':
			capture = false
			p.position++
		case '=', '!':
			capture = false
			frame.look = true
			frame.negative = p.text[p.position] == '!'
			p.position++
		case '<':
			p.position++
			if p.position < len(p.text) && (p.text[p.position] == '=' || p.text[p.position] == '!') {
				capture = false
				frame.look = true
				frame.backward = true
				frame.negative = p.text[p.position] == '!'
				p.position++
			} else {
				var err error
				name, err = p.groupName()
				if err != nil {
					return err
				}
			}
		default:
			capture = false
			flags, err := p.modifiers(parent.flags)
			if err != nil {
				return err
			}
			frame.flags = flags
		}
	}
	if capture {
		p.program.Groups++
		frame.capture = p.program.Groups
		if name != "" {
			p.names[name] = append(p.names[name], declaration{group: frame.capture, path: frame.outerPath})
		}
	}
	p.frames = append(p.frames, frame)
	return nil
}
func (p *compiler) closeGroup() error {
	p.position++
	frame := p.frames[len(p.frames)-1]
	p.frames = p.frames[:len(p.frames)-1]
	child := p.finish(frame)
	first, last := 0, 0
	if frame.firstGroup <= p.program.Groups {
		first, last = frame.firstGroup, p.program.Groups
	}
	if frame.capture != 0 {
		child = p.node(Instruction{Op: Capture, A: child, Group: frame.capture, FirstGroup: first, LastGroup: last})
	}
	if frame.capture == 0 && !frame.look {
		offset := len(p.program.Edges)
		p.program.Edges = append(p.program.Edges, child)
		child = p.node(Instruction{Op: Sequence, A: offset, B: 1, FirstGroup: first, LastGroup: last})
	}
	if frame.look {
		child = p.node(Instruction{Op: Look, A: child, Negative: frame.negative, Backward: frame.backward, FirstGroup: first, LastGroup: last})
	}
	p.push(child)
	return nil
}
func flag(value rune) Flags {
	switch value {
	case 'i':
		return IgnoreCase
	case 'm':
		return Multiline
	case 's':
		return DotAll
	}
	return 0
}
func (p *compiler) modifiers(inherited Flags) (Flags, error) {
	var added, removed, seen Flags
	negative := false
	count := 0
	for p.position < len(p.text) {
		r := p.text[p.position]
		p.position++
		if r == ':' {
			if count == 0 {
				return 0, p.invalid("empty modifier group")
			}
			return (inherited | added) &^ removed, nil
		}
		if r == '-' && !negative {
			negative = true
			continue
		}
		f := flag(r)
		if f == 0 || seen&f != 0 {
			return 0, p.invalid("invalid modifier group")
		}
		seen |= f
		count++
		if negative {
			removed |= f
		} else {
			added |= f
		}
	}
	return 0, p.invalid("unterminated modifier group")
}
func (p *compiler) groupName() (string, error) {
	var result []rune
	start, _ := unicode17.LookupProperty("", "ID_Start")
	continuation, _ := unicode17.LookupProperty("", "ID_Continue")
	for p.position < len(p.text) && p.text[p.position] != '>' {
		if err := p.step(); err != nil {
			return "", err
		}
		r := p.text[p.position]
		p.position++
		if r == '\\' {
			if p.position == len(p.text) || p.text[p.position] != 'u' {
				return "", p.invalid("invalid group name escape")
			}
			p.position++
			var err error
			r, err = p.unicodeEscape()
			if err != nil {
				return "", err
			}
		}
		valid := r == '$' || r == '_' || start.Contains(r)
		if len(result) > 0 {
			valid = r == '$' || continuation.Contains(r) || r == 0x200c || r == 0x200d
		}
		if !valid {
			return "", p.invalid("invalid capture name")
		}
		result = append(result, r)
	}
	if p.position == len(p.text) || len(result) == 0 {
		return "", p.invalid("unterminated or empty capture name")
	}
	p.position++
	return string(result), nil
}
func (p *compiler) exclusive(a, b int) (bool, error) {
	for a >= 0 && b >= 0 {
		if err := p.step(); err != nil {
			return false, err
		}
		left, right := p.paths[a], p.paths[b]
		if left.alt == right.alt {
			if left.branch != right.branch {
				return true, nil
			}
			a, b = left.parent, right.parent
		} else if left.depth > right.depth {
			a = left.parent
		} else if right.depth > left.depth {
			b = right.parent
		} else {
			a, b = left.parent, right.parent
		}
	}
	return false, nil
}
func (p *compiler) resolveNames() error {
	names := make([]string, 0, len(p.names))
	for name := range p.names {
		names = append(names, name)
	}
	sort.Strings(names)
	ids := make(map[string]int)
	for _, name := range names {
		declarations := p.names[name]
		for i := range declarations {
			for j := 0; j < i; j++ {
				ok, err := p.exclusive(declarations[i].path, declarations[j].path)
				if err != nil {
					return err
				}
				if !ok {
					return p.invalid("duplicate capture name")
				}
			}
		}
		named := NamedCapture{Name: name}
		for _, d := range declarations {
			named.Groups = append(named.Groups, d.group)
		}
		ids[name] = len(p.program.Named)
		p.program.Named = append(p.program.Named, named)
	}
	for _, ref := range p.refs {
		node := &p.program.Nodes[ref.node]
		if ref.name != "" {
			id, ok := ids[ref.name]
			if !ok {
				return p.invalid("unknown named capture reference")
			}
			node.A = id
		} else if node.Group > p.program.Groups {
			return p.invalid("invalid numeric capture reference")
		}
	}
	return nil
}
func (p *compiler) atom() (int, error) {
	r := p.text[p.position]
	p.position++
	flags := p.top().flags
	switch r {
	case '[':
		return p.characterClass(flags)
	case '\\':
		return p.escapeAtom(flags)
	case '.':
		return p.node(Instruction{Op: Any, Flags: flags}), nil
	case '^':
		return p.node(Instruction{Op: Start, Flags: flags}), nil
	case '$':
		return p.node(Instruction{Op: End, Flags: flags}), nil
	case ']', '}':
		return 0, p.invalid("unescaped syntax character")
	}
	return p.node(Instruction{Op: Literal, Value: r, Flags: flags}), nil
}
func (p *compiler) quantifier() error {
	frame := p.top()
	if len(frame.sequence) == 0 {
		return p.invalid("nothing to repeat")
	}
	index := frame.sequence[len(frame.sequence)-1]
	child := p.program.Nodes[index]
	if child.Op == Repeat || child.Op == Look || child.Op == Start || child.Op == End || child.Op == WordBoundary {
		return p.invalid("invalid quantified assertion or repetition")
	}
	r := p.text[p.position]
	p.position++
	var minimum, maximum uint64
	switch r {
	case '*':
		maximum = math.MaxUint64
	case '+':
		minimum = 1
		maximum = math.MaxUint64
	case '?':
		maximum = 1
	case '{':
		low, err := p.decimal()
		if err != nil {
			return err
		}
		high := low
		if p.position < len(p.text) && p.text[p.position] == ',' {
			p.position++
			if p.position < len(p.text) && p.text[p.position] == '}' {
				high = ""
			} else {
				high, err = p.decimal()
				if err != nil {
					return err
				}
			}
		}
		if p.position == len(p.text) || p.text[p.position] != '}' {
			return p.invalid("unterminated quantifier")
		}
		p.position++
		if high != "" && (len(low) > len(high) || len(low) == len(high) && low > high) {
			return p.invalid("quantifier bounds out of order")
		}
		minimum = decimalValue(low)
		maximum = math.MaxUint64
		if high != "" {
			maximum = decimalValue(high)
		}
	}
	lazy := false
	if p.position < len(p.text) && p.text[p.position] == '?' {
		lazy = true
		p.position++
	}
	repeated := p.node(Instruction{Op: Repeat, A: index, Min: minimum, Max: maximum, Lazy: lazy, FirstGroup: child.FirstGroup, LastGroup: child.LastGroup})
	p.top().sequence[len(p.top().sequence)-1] = repeated
	return nil
}
func (p *compiler) decimal() (string, error) {
	start := p.position
	for p.position < len(p.text) && p.text[p.position] >= '0' && p.text[p.position] <= '9' {
		p.position++
		if err := p.step(); err != nil {
			return "", err
		}
	}
	if start == p.position {
		return "", p.invalid("expected decimal digits")
	}
	result := strings.TrimLeft(string(p.text[start:p.position]), "0")
	if result == "" {
		result = "0"
	}
	return result, nil
}
func decimalValue(text string) uint64 {
	var result uint64
	for _, r := range text {
		n := uint64(r - '0')
		if result > (math.MaxUint64-n)/10 {
			return math.MaxUint64
		}
		result = result*10 + n
	}
	return result
}
