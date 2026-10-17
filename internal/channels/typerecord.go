package channels

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
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
// keys are read, and the message properties whose stored values are entity
// ids. Boot compares the record with realtime.yaml through
// [TypeRecordChanges].
//
// Refs only grow. A property stored as a ref holds entity ids, which a
// reader who cannot see the entity must never get as plain text, so once a
// ref, always a ref: a record whose Plain names it conflicts. A property
// that is no longer declared at all is harmless, because the channel
// service renders only the properties the configuration declares.
type TypeRecord struct {
	Anchor      AnchorMode  `yaml:"anchor"`
	Cardinality Cardinality `yaml:"cardinality"`
	// Refs lists the ref properties of the type's message types, as
	// "message-type.property", sorted.
	Refs []string `yaml:"refs,omitempty"`
	// Plain lists the declared properties that are not refs, in the same
	// form. [Store.RecordTypes] checks it against the held refs and does
	// not store it, so a record read back has none.
	Plain []string `yaml:"-"`
}

// Equal reports whether r and o are the same record.
func (r TypeRecord) Equal(o TypeRecord) bool {
	return r.Anchor == o.Anchor && r.Cardinality == o.Cardinality &&
		slices.Equal(r.Refs, o.Refs) && slices.Equal(r.Plain, o.Plain)
}

// PrepareTypeRecords validates records for [Store.RecordTypes] and returns
// a copy with Refs and Plain sorted and deduplicated. Each key must be a
// valid channel type, each record a known mode and cardinality, and each
// property non-empty text of at most [MaxTextBytes] bytes.
func PrepareTypeRecords(records map[string]TypeRecord) (map[string]TypeRecord, error) {
	out := make(map[string]TypeRecord, len(records))
	for typ, r := range records {
		if !ValidName(typ) {
			return nil, fmt.Errorf("%w: type %q", ErrInvalidKey, typ)
		}
		if !r.Anchor.Valid() {
			return nil, fmt.Errorf("%w: anchor mode %q for type %s", ErrInvalidMessage, r.Anchor, typ)
		}
		if !r.Cardinality.Valid() {
			return nil, fmt.Errorf("%w: cardinality %q for type %s", ErrInvalidMessage, r.Cardinality, typ)
		}
		for _, p := range slices.Concat(r.Refs, r.Plain) {
			if p == "" || len(p) > MaxTextBytes || strings.ContainsFunc(p, unicode.IsControl) {
				return nil, fmt.Errorf("%w: property %q for type %s", ErrInvalidMessage, p, typ)
			}
		}
		r.Refs, r.Plain = sortedSet(r.Refs), sortedSet(r.Plain)
		out[typ] = r
	}
	return out, nil
}

func sortedSet(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// MergeTypeRecord merges given into the record the store holds for typ, by
// the rule of [Store.RecordTypes], and returns the refs of given that the
// store must add. held reports whether the store holds a record; when it
// does not, the result is every ref of given and given is recorded whole.
//
// A held record whose anchor mode or cardinality differs from given, or
// one of whose refs given declares in Plain, is [ErrTypeShapeConflict].
// Both records must have passed [PrepareTypeRecords] or come from the
// store, so their refs are sorted.
func MergeTypeRecord(typ string, cur TypeRecord, held bool, given TypeRecord) ([]string, error) {
	if !held {
		return given.Refs, nil
	}
	if cur.Anchor != given.Anchor || cur.Cardinality != given.Cardinality {
		return nil, fmt.Errorf("%w: %s: anchor %s and cardinality %s are recorded, %s and %s given",
			ErrTypeShapeConflict, typ, cur.Anchor, cur.Cardinality, given.Anchor, given.Cardinality)
	}
	for _, p := range given.Plain {
		if _, found := slices.BinarySearch(cur.Refs, p); found {
			return nil, fmt.Errorf("%w: %s: property %s is recorded as a ref and declared as another type",
				ErrTypeShapeConflict, typ, p)
		}
	}
	var added []string
	for _, r := range given.Refs {
		if _, found := slices.BinarySearch(cur.Refs, r); !found {
			added = append(added, r)
		}
	}
	return added, nil
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
// sorted by type. Refs are not compared; [Store.RecordTypes] and the
// realtime configuration check hold them. A type that is configured but
// not recorded has no stored channels yet, and a type that is recorded but
// no longer configured is unused; neither is a change.
func TypeRecordChanges(recorded, configured map[string]TypeRecord) []TypeRecordChange {
	var out []TypeRecordChange
	for typ, want := range configured {
		got, ok := recorded[typ]
		if ok && (got.Anchor != want.Anchor || got.Cardinality != want.Cardinality) {
			out = append(out, TypeRecordChange{Type: typ, Recorded: got, Configured: want})
		}
	}
	slices.SortFunc(out, func(a, b TypeRecordChange) int { return strings.Compare(a.Type, b.Type) })
	return out
}
