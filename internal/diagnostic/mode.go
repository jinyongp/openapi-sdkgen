package diagnostic

import "fmt"

// Mode controls how far diagnostic analysis continues after an expected
// author/input/target finding is discovered.
type Mode string

const (
	ModeFailFast Mode = "fail-fast"
	ModeCollect  Mode = "collect"
)

// Resolve returns the canonical mode. The zero value preserves historical
// fail-fast behavior for existing direct compiler and target callers.
func (mode Mode) Resolve() (Mode, error) {
	if mode == "" {
		return ModeFailFast, nil
	}
	switch mode {
	case ModeFailFast, ModeCollect:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported diagnostic mode %q", mode)
	}
}
