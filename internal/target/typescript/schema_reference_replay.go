package typescript

import "fmt"

type schemaProjectionReference struct {
	name      string
	direction projection
}

// schemaReferenceReplay checks the semantic callback sequence, including self
// references. A matching occurrence count alone cannot detect swapped schemas
// or input/output projections between the collection and rendering passes.
type schemaReferenceReplay struct {
	owner    string
	expected []schemaProjectionReference
	next     int
}

func (replay *schemaReferenceReplay) observe(name string, direction projection) error {
	actual := schemaProjectionReference{name: name, direction: direction}
	if replay.next >= len(replay.expected) {
		return fmt.Errorf("schema references for %q: unplanned reference %q projection %q at %d", replay.owner, name, direction, replay.next)
	}
	wanted := replay.expected[replay.next]
	if wanted != actual {
		return fmt.Errorf("schema references for %q: reference %d changed from %q projection %q to %q projection %q", replay.owner, replay.next, wanted.name, wanted.direction, name, direction)
	}
	replay.next++
	return nil
}

func (replay *schemaReferenceReplay) finish() error {
	if replay.next != len(replay.expected) {
		return fmt.Errorf("schema references for %q: rendered %d references, planned %d", replay.owner, replay.next, len(replay.expected))
	}
	return nil
}
