package ecmaregexp

// Escape analysis moves the VM's four buffers to the heap. Reuse a bounded
// number of exclusively borrowed states; never retain grown buffers or input.
type matchScratch struct {
	state    machine
	tasks    [128]task
	choices  [32]choice
	journal  [128]change
	captures [64]int
}

var scratchCache = make(chan *matchScratch, 32)

func borrowScratch() *matchScratch {
	select {
	case scratch := <-scratchCache:
		return scratch
	default:
		return new(matchScratch)
	}
}
func resetScratch(scratch *matchScratch) {
	// Tasks/choices/journal contain only integers. Clearing the state releases
	// input, Program, context and any grown heap buffers between callers.
	scratch.state = machine{}
}
func returnScratch(scratch *matchScratch) {
	resetScratch(scratch)
	select {
	case scratchCache <- scratch:
	default:
	}
}
