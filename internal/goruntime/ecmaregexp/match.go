package ecmaregexp

import (
	"context"
	"fmt"
	"openapi-sdkgen/internal/goruntime/unicode17"
)

type actionKind uint8

const (
	visitNode actionKind = iota
	captureDone
	repeatContinue
	repeatDone
	assertionDone
	assertionFailed
	simpleShorter
	simpleLonger
	proceed
)

type action struct {
	kind                                         actionKind
	node                                         int
	count                                        uint64
	position, direction, choiceBase, journalBase int
}
type task struct {
	action action
	next   int
}
type choice struct {
	action                                    action
	head, arena, journal, position, direction int
}
type change struct{ index, value int }
type machine struct {
	program                   *Program
	input                     string
	context                   context.Context
	steps, limit              uint64
	position, direction, head int
	tasks                     []task
	choices                   []choice
	captures                  []int
	journal                   []change
}

func (program *Program) Match(ctx context.Context, value string) (bool, error) {
	return program.match(ctx, value, MaxSteps)
}
func (program *Program) match(ctx context.Context, value string, limit uint64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if program == nil || program.Version != Version {
		return false, fmt.Errorf("invalid regular expression program version")
	}
	scratch := borrowScratch()
	defer returnScratch(scratch)
	state := &scratch.state
	*state = machine{program: program, input: value, context: ctx, limit: limit, tasks: scratch.tasks[:0], choices: scratch.choices[:0], journal: scratch.journal[:0]}
	count := 2 * (program.Groups + 1)
	if count <= len(scratch.captures) {
		state.captures = scratch.captures[:count]
	} else {
		state.captures = make([]int, count)
	}
	for start := 0; ; {
		state.position, state.direction, state.head = start, 1, -1
		state.tasks = state.tasks[:0]
		state.choices = state.choices[:0]
		state.journal = state.journal[:0]
		for i := range state.captures {
			if err := state.step(); err != nil {
				return false, err
			}
			state.captures[i] = -1
		}
		state.push(action{kind: visitNode, node: program.Root})
		for {
			if state.head < 0 {
				if err := ctx.Err(); err != nil {
					return false, err
				}
				return true, nil
			}
			if err := state.step(); err != nil {
				return false, err
			}
			current := state.pop()
			ok, err := state.execute(current)
			if err != nil {
				return false, err
			}
			if !ok {
				resumed, err := state.retry()
				if err != nil {
					return false, err
				}
				if !resumed {
					break
				}
			}
		}
		if start == len(value) {
			break
		}
		_, next, _ := forward(value, start)
		start = next
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, nil
}
func (state *machine) step() error {
	if state.steps >= state.limit {
		return ErrComplexity
	}
	state.steps++
	if state.steps%1024 == 0 {
		return state.context.Err()
	}
	return nil
}
func (state *machine) reclaim() {
	protected := 0
	if len(state.choices) > 0 {
		protected = state.choices[len(state.choices)-1].arena
	}
	end := max(state.head+1, protected)
	if end < len(state.tasks) {
		state.tasks = state.tasks[:end]
	}
}
func (state *machine) push(value action) {
	state.reclaim()
	state.tasks = append(state.tasks, task{action: value, next: state.head})
	state.head = len(state.tasks) - 1
}
func (state *machine) pop() action {
	current := state.tasks[state.head]
	state.head = current.next
	state.reclaim()
	return current.action
}
func (state *machine) save(value action) {
	state.choices = append(state.choices, choice{action: value, head: state.head, arena: len(state.tasks), journal: len(state.journal), position: state.position, direction: state.direction})
}
func (state *machine) rollback(checkpoint int) error {
	for len(state.journal) > checkpoint {
		if err := state.step(); err != nil {
			return err
		}
		last := state.journal[len(state.journal)-1]
		state.captures[last.index] = last.value
		state.journal = state.journal[:len(state.journal)-1]
	}
	return nil
}
func (state *machine) retry() (bool, error) {
	if len(state.choices) == 0 {
		return false, nil
	}
	last := state.choices[len(state.choices)-1]
	state.choices = state.choices[:len(state.choices)-1]
	if err := state.rollback(last.journal); err != nil {
		return false, err
	}
	state.tasks = state.tasks[:last.arena]
	state.head = last.head
	state.position, state.direction = last.position, last.direction
	state.push(last.action)
	return true, nil
}
func (state *machine) store(index, value int) error {
	if err := state.step(); err != nil {
		return err
	}
	if state.captures[index] != value {
		state.journal = append(state.journal, change{index: index, value: state.captures[index]})
		state.captures[index] = value
	}
	return nil
}
func (state *machine) clear(node Instruction) error {
	if node.FirstGroup == 0 {
		return nil
	}
	for group := node.FirstGroup; group <= node.LastGroup; group++ {
		if err := state.store(2*group, -1); err != nil {
			return err
		}
		if err := state.store(2*group+1, -1); err != nil {
			return err
		}
	}
	return nil
}
func lineTerminator(value rune) bool {
	return value == '\n' || value == '\r' || value == 0x2028 || value == 0x2029
}
func word(value rune, flags Flags) bool {
	if flags&IgnoreCase != 0 {
		value = unicode17.Fold(value)
	}
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}
func equal(left, right rune, flags Flags) bool {
	if flags&IgnoreCase != 0 {
		return unicode17.Fold(left) == unicode17.Fold(right)
	}
	return left == right
}
func (state *machine) accepts(node Instruction, value rune) bool {
	switch node.Op {
	case Literal:
		return equal(node.Value, value, node.Flags)
	case Any:
		return node.Flags&DotAll != 0 || !lineTerminator(value)
	case CharacterClass:
		if node.Flags&IgnoreCase != 0 {
			value = unicode17.Fold(value)
		}
		return rangeContains(state.program.Ranges[node.A:node.A+node.B], value)
	}
	return false
}
func (state *machine) single(index int) (int, bool, error) {
	for {
		if err := state.step(); err != nil {
			return 0, false, err
		}
		node := state.program.Nodes[index]
		if node.Op == Sequence && node.B == 1 && node.FirstGroup == 0 {
			index = state.program.Edges[node.A]
			continue
		}
		return index, node.Op == Literal || node.Op == CharacterClass || node.Op == Any, nil
	}
}
func (state *machine) consume(index int) (bool, error) {
	if err := state.step(); err != nil {
		return false, err
	}
	value, next, ok := readPoint(state.input, state.position, state.direction)
	if !ok || !state.accepts(state.program.Nodes[index], value) {
		return false, nil
	}
	state.position = next
	return true, nil
}
func (state *machine) simpleRepeat(index, atom int) (bool, error) {
	node := state.program.Nodes[index]
	var count uint64
	for count < node.Min {
		ok, err := state.consume(atom)
		if err != nil || !ok {
			return ok, err
		}
		count++
	}
	if node.Lazy {
		if count < node.Max {
			state.save(action{kind: simpleLonger, node: index, count: count})
		}
		return true, nil
	}
	for count < node.Max {
		ok, err := state.consume(atom)
		if err != nil {
			return false, err
		}
		if !ok {
			break
		}
		count++
	}
	if count > node.Min {
		state.save(action{kind: simpleShorter, node: index, count: count})
	}
	return true, nil
}
func (state *machine) iterate(index int, count uint64) error {
	node := state.program.Nodes[index]
	if err := state.clear(node); err != nil {
		return err
	}
	state.push(action{kind: repeatDone, node: index, count: count, position: state.position})
	state.push(action{kind: visitNode, node: node.A})
	return nil
}
func (state *machine) repeat(index int, count uint64) (bool, error) {
	node := state.program.Nodes[index]
	if count == node.Max {
		return true, nil
	}
	if count >= node.Min {
		if node.Lazy {
			state.save(action{kind: repeatContinue, node: index, count: count})
			return true, nil
		}
		state.save(action{kind: proceed})
	}
	return true, state.iterate(index, count)
}
func (state *machine) reference(node Instruction) (bool, error) {
	group := node.Group
	if group == 0 {
		for _, candidate := range state.program.Named[node.A].Groups {
			if state.captures[candidate*2] >= 0 && state.captures[candidate*2+1] >= 0 {
				group = candidate
				break
			}
		}
		if group == 0 {
			return true, nil
		}
	}
	start, end := state.captures[group*2], state.captures[group*2+1]
	if start < 0 || end < 0 {
		return true, nil
	}
	cursor := start
	if state.direction < 0 {
		cursor = end
	}
	for cursor != end && state.direction > 0 || cursor != start && state.direction < 0 {
		if err := state.step(); err != nil {
			return false, err
		}
		expected, next, _ := readPoint(state.input, cursor, state.direction)
		actual, following, ok := readPoint(state.input, state.position, state.direction)
		if !ok || !equal(expected, actual, node.Flags) {
			return false, nil
		}
		cursor = next
		state.position = following
	}
	return true, nil
}
func (state *machine) execute(current action) (bool, error) {
	switch current.kind {
	case proceed:
		return true, nil
	case captureDone:
		index := current.node*2 + 1
		if current.direction < 0 {
			index = current.node * 2
		}
		return true, state.store(index, state.position)
	case repeatContinue:
		return true, state.iterate(current.node, current.count)
	case repeatDone:
		node := state.program.Nodes[current.node]
		if state.position == current.position && current.count >= node.Min {
			return false, nil
		}
		return state.repeat(current.node, current.count+1)
	case assertionDone:
		state.choices = state.choices[:current.choiceBase]
		state.reclaim()
		state.position, state.direction = current.position, current.direction
		node := state.program.Nodes[current.node]
		if node.Negative {
			if err := state.rollback(current.journalBase); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	case assertionFailed:
		return state.program.Nodes[current.node].Negative, nil
	case simpleShorter:
		node := state.program.Nodes[current.node]
		_, next, ok := readPoint(state.input, state.position, -state.direction)
		if !ok {
			return false, nil
		}
		state.position = next
		count := current.count - 1
		if count > node.Min {
			state.save(action{kind: simpleShorter, node: current.node, count: count})
		}
		return true, nil
	case simpleLonger:
		atom, _, err := state.single(state.program.Nodes[current.node].A)
		if err != nil {
			return false, err
		}
		ok, err := state.consume(atom)
		if err != nil || !ok {
			return ok, err
		}
		count := current.count + 1
		if count < state.program.Nodes[current.node].Max {
			state.save(action{kind: simpleLonger, node: current.node, count: count})
		}
		return true, nil
	}
	node := state.program.Nodes[current.node]
	switch node.Op {
	case Empty:
		return true, nil
	case Literal, CharacterClass, Any:
		return state.consume(current.node)
	case Sequence:
		if state.direction > 0 {
			for i := node.B - 1; i >= 0; i-- {
				state.push(action{kind: visitNode, node: state.program.Edges[node.A+i]})
			}
		} else {
			for i := 0; i < node.B; i++ {
				state.push(action{kind: visitNode, node: state.program.Edges[node.A+i]})
			}
		}
		return true, nil
	case Alternative:
		for i := node.B - 1; i > 0; i-- {
			state.save(action{kind: visitNode, node: state.program.Edges[node.A+i]})
		}
		state.push(action{kind: visitNode, node: state.program.Edges[node.A]})
		return true, nil
	case Capture:
		start, end := node.Group*2, node.Group*2+1
		if state.direction < 0 {
			start, end = end, start
		}
		if err := state.store(start, state.position); err != nil {
			return false, err
		}
		if err := state.store(end, -1); err != nil {
			return false, err
		}
		state.push(action{kind: captureDone, node: node.Group, direction: state.direction})
		state.push(action{kind: visitNode, node: node.A})
		return true, nil
	case Reference:
		return state.reference(node)
	case Look:
		base, journal := len(state.choices), len(state.journal)
		position, direction := state.position, state.direction
		state.save(action{kind: assertionFailed, node: current.node})
		state.push(action{kind: assertionDone, node: current.node, choiceBase: base, journalBase: journal, position: position, direction: direction})
		state.direction = 1
		if node.Backward {
			state.direction = -1
		}
		state.push(action{kind: visitNode, node: node.A})
		return true, nil
	case Repeat:
		index, single, err := state.single(node.A)
		if err != nil {
			return false, err
		}
		if single {
			return state.simpleRepeat(current.node, index)
		}
		return state.repeat(current.node, 0)
	case Start:
		if state.position == 0 {
			return true, nil
		}
		previous, _, _ := backward(state.input, state.position)
		return node.Flags&Multiline != 0 && lineTerminator(previous), nil
	case End:
		if state.position == len(state.input) {
			return true, nil
		}
		next, _, _ := forward(state.input, state.position)
		return node.Flags&Multiline != 0 && lineTerminator(next), nil
	case WordBoundary:
		left, right := false, false
		if r, _, ok := backward(state.input, state.position); ok {
			left = word(r, node.Flags)
		}
		if r, _, ok := forward(state.input, state.position); ok {
			right = word(r, node.Flags)
		}
		return (left != right) != node.Negative, nil
	}
	return false, fmt.Errorf("invalid regular expression opcode")
}
