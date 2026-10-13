package channels

import (
	"fmt"
	"slices"
	"strings"
)

// AnchorMode says what a channel type anchors to: the entity family, shared
// by every face, or one face. realtime.yaml declares it per type. The store
// records the mode each type was used with, because changing it would
// reinterpret every stored key: family channels have an empty face, face
// channels a named one.
type AnchorMode string

// The anchor modes.
const (
	AnchorFamily AnchorMode = "family"
	AnchorFace   AnchorMode = "face"
)

// Valid reports whether m is a known mode.
func (m AnchorMode) Valid() bool { return m == AnchorFamily || m == AnchorFace }

// PrepareAnchorModes validates modes for [Store.RecordAnchorModes]: each
// key must be a valid channel type and each value a known mode.
func PrepareAnchorModes(modes map[string]AnchorMode) error {
	for typ, m := range modes {
		if !ValidName(typ) {
			return fmt.Errorf("%w: type %q", ErrInvalidKey, typ)
		}
		if !m.Valid() {
			return fmt.Errorf("%w: anchor mode %q for type %s", ErrInvalidMessage, m, typ)
		}
	}
	return nil
}

// AnchorModeChange is a channel type whose configured mode differs from
// the mode the store recorded for it.
type AnchorModeChange struct {
	Type       string
	Recorded   AnchorMode
	Configured AnchorMode
}

// AnchorModeChanges compares the configured modes against the recorded
// ones and returns the types whose mode changed, sorted by type. A type
// that is configured but not recorded is new, and a type that is recorded
// but no longer configured is unused; neither is a change.
func AnchorModeChanges(recorded, configured map[string]AnchorMode) []AnchorModeChange {
	var out []AnchorModeChange
	for typ, want := range configured {
		if got, ok := recorded[typ]; ok && got != want {
			out = append(out, AnchorModeChange{Type: typ, Recorded: got, Configured: want})
		}
	}
	slices.SortFunc(out, func(a, b AnchorModeChange) int { return strings.Compare(a.Type, b.Type) })
	return out
}
