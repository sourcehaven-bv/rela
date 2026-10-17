package channels

import (
	"fmt"
	"slices"
	"strings"
)

// AnchorMode says what a channel type anchors to: the entity family, shared
// by every face, or one face. realtime.yaml declares it per type. Family
// channels have an empty face in their key, face channels a named one, so a
// changed mode would reinterpret every stored key.
type AnchorMode string

// The anchor modes.
const (
	AnchorFamily AnchorMode = "family"
	AnchorFace   AnchorMode = "face"
)

// Valid reports whether m is a known mode.
func (m AnchorMode) Valid() bool { return m == AnchorFamily || m == AnchorFace }

// Cardinality says how many channels of a type one anchor holds: one
// implicit channel with an empty name, or any number of named ones. A
// changed cardinality would strand every stored channel of the type, since
// a "one" type never reads a named channel and a "many" type never reads
// the unnamed one.
type Cardinality string

// The cardinalities.
const (
	CardinalityOne  Cardinality = "one"
	CardinalityMany Cardinality = "many"
)

// Valid reports whether c is a known cardinality.
func (c Cardinality) Valid() bool { return c == CardinalityOne || c == CardinalityMany }

// TypeRecord is what the store records about a channel type the first time
// a channel of that type is written: the two settings that decide how its
// keys are read. Boot compares the record with realtime.yaml through
// [TypeRecordChanges].
type TypeRecord struct {
	Anchor      AnchorMode  `yaml:"anchor"`
	Cardinality Cardinality `yaml:"cardinality"`
}

// PrepareTypeRecords validates records for [Store.RecordTypes]: each key
// must be a valid channel type, and each record a known mode and
// cardinality.
func PrepareTypeRecords(records map[string]TypeRecord) error {
	for typ, r := range records {
		if !ValidName(typ) {
			return fmt.Errorf("%w: type %q", ErrInvalidKey, typ)
		}
		if !r.Anchor.Valid() {
			return fmt.Errorf("%w: anchor mode %q for type %s", ErrInvalidMessage, r.Anchor, typ)
		}
		if !r.Cardinality.Valid() {
			return fmt.Errorf("%w: cardinality %q for type %s", ErrInvalidMessage, r.Cardinality, typ)
		}
	}
	return nil
}

// ChangedTypeRecords returns the entries of next that differ from cur: the
// ones [Store.RecordTypes] must write. Backends use it so a record that did
// not change costs no write.
func ChangedTypeRecords(cur, next map[string]TypeRecord) map[string]TypeRecord {
	out := map[string]TypeRecord{}
	for typ, r := range next {
		if got, ok := cur[typ]; !ok || got != r {
			out[typ] = r
		}
	}
	return out
}

// TypeRecordChange is a channel type whose configured settings differ from
// the ones the store recorded for it.
type TypeRecordChange struct {
	Type       string
	Recorded   TypeRecord
	Configured TypeRecord
}

// TypeRecordChanges compares the configured records against the recorded
// ones and returns the types whose anchor mode or cardinality changed,
// sorted by type. A type that is configured but not recorded has no stored
// channels yet, and a type that is recorded but no longer configured is
// unused; neither is a change.
func TypeRecordChanges(recorded, configured map[string]TypeRecord) []TypeRecordChange {
	var out []TypeRecordChange
	for typ, want := range configured {
		if got, ok := recorded[typ]; ok && got != want {
			out = append(out, TypeRecordChange{Type: typ, Recorded: got, Configured: want})
		}
	}
	slices.SortFunc(out, func(a, b TypeRecordChange) int { return strings.Compare(a.Type, b.Type) })
	return out
}
