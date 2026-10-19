package ecmaregexp

import (
	"sync"
	"sync/atomic"
)

// Buffer storage is bounded independently of sync.Pool. Pool entries are only
// hints: GC may discard them, and scanning can leave a stale hint. A successful
// lease CAS is the sole authority to use a buffer. Overflow states are temporary.
type matchScratch struct {
	state    machine
	tasks    [128]task
	choices  [32]choice
	journal  [128]change
	captures [64]int
	leased   atomic.Bool
	cached   bool
}

var scratchBuffers [32]matchScratch
var scratchHints sync.Pool

func borrowScratch() *matchScratch {
	// Bound stale-hint work; scanning the fixed arena also recovers buffers after
	// GC drops pool entries. The common per-P pool path avoids a global channel.
	for range len(scratchBuffers) {
		hint := scratchHints.Get()
		if hint == nil {
			break
		}
		scratch := hint.(*matchScratch)
		if scratch.leased.CompareAndSwap(false, true) {
			return scratch
		}
	}
	for index := range scratchBuffers {
		scratch := &scratchBuffers[index]
		if scratch.leased.CompareAndSwap(false, true) {
			scratch.cached = true
			return scratch
		}
	}
	return new(matchScratch)
}
func resetScratch(scratch *matchScratch) {
	// Integer arrays contain no input data. Release input, Program, context and
	// any grown heap buffers before publishing an available lease.
	scratch.state = machine{}
}
func returnScratch(scratch *matchScratch) {
	resetScratch(scratch)
	if scratch.cached {
		scratch.leased.Store(false)
		scratchHints.Put(scratch)
	}
}
