package channels

import (
	"errors"
	"fmt"
	"slices"
)

// ErrTooManyRows reports a listing whose matches exceed its row ceiling.
// Nothing is returned: a partial answer would read as a complete one.
var ErrTooManyRows = errors.New("channels: too many rows")

// The bounds of [Store.ListWaiting].
const (
	// MaxWaitingAnchors is the most anchors one [WaitingQuery] may name.
	MaxWaitingAnchors = 20000
	// MaxWaitingRows is the most distinct channels one
	// [Store.ListWaiting] call returns; more is [ErrTooManyRows]. A query
	// may set a lower ceiling in [WaitingQuery.MaxRows].
	MaxWaitingRows = 20000
)

// WaitingQuery selects the channels an inbox looks at: open, live channels
// of a type in States, in a listed stored session state, that are on one
// of Anchors or were created by CreatedBy as a type in CreatorTypes.
//
// The result is a superset of what the inbox shows; the caller decides each
// channel against the configuration and the reader.
type WaitingQuery struct {
	// States maps a channel type to the stored session states to list. ""
	// is a session that never moved. A type absent here is not listed.
	States map[string][]string
	// Anchors are exact anchors: the id and the face. Face "" is a
	// family-anchored channel, never every face.
	Anchors []Anchor
	// CreatorTypes are the channel types the creator branch covers.
	CreatorTypes []string
	// CreatedBy is the [Author.User] of the creator. "" matches no
	// channel, so the creator branch is off.
	CreatedBy string
	// MaxRows is the row ceiling of this call, from 1 to [MaxWaitingRows].
	// 0 is [MaxWaitingRows].
	MaxRows int
	// Names narrows both branches by channel name. The zero value lists
	// every name.
	Names NameFilter
}

// NameFilter narrows a [WaitingQuery] by channel name, so a caller that
// knows a type's cardinality never counts a channel it cannot address.
// A type switched between one and many keeps the channels of its old
// cardinality, and those would otherwise count toward the row ceiling.
type NameFilter int

const (
	// AnyName lists every channel.
	AnyName NameFilter = iota
	// UnnamedOnly lists only unnamed channels, the one channel of a
	// cardinality one type.
	UnnamedOnly
	// NamedOnly lists only named channels, those of a cardinality many
	// type.
	NamedOnly
)

// Admits reports whether a channel named name passes f.
func (f NameFilter) Admits(name string) bool {
	switch f {
	case UnnamedOnly:
		return name == ""
	case NamedOnly:
		return name != ""
	default:
		return true
	}
}

// RowCeiling is the most distinct channels q may return.
func (q WaitingQuery) RowCeiling() int {
	if q.MaxRows == 0 {
		return MaxWaitingRows
	}
	return q.MaxRows
}

// CreatorBranch reports whether q's creator branch can match anything.
func (q WaitingQuery) CreatorBranch() bool { return q.CreatedBy != "" && len(q.CreatorTypes) > 0 }

// Matches reports whether c is a channel q selects, judged on the stored
// row: live, open, a listed type and state, and on a listed anchor or made
// by the creator. A backend that filters in Go uses it, so the backends
// agree on what a match is.
func (q WaitingQuery) Matches(c Channel, anchors map[Anchor]bool) bool {
	states, ok := q.States[c.Key.Type]
	if !ok || c.State != StateOpen || !slices.Contains(states, c.Session.State) || !q.Names.Admits(c.Key.Name) {
		return false
	}
	if anchors[c.Key.Anchor] {
		return true
	}
	return q.CreatorBranch() && c.CreatedBy.User == q.CreatedBy && slices.Contains(q.CreatorTypes, c.Key.Type)
}

// CheckWaitingQuery validates the arguments of [Store.ListWaiting], each
// failure [ErrInvalidKey]: at least one type with at least one state, every
// type and state valid, anchors or a creator branch, every anchor valid,
// at most [MaxWaitingAnchors] anchors, every creator type valid, a
// MaxRows from 0 to [MaxWaitingRows], and a known [NameFilter].
func CheckWaitingQuery(q WaitingQuery) error {
	if q.MaxRows < 0 || q.MaxRows > MaxWaitingRows {
		return fmt.Errorf("%w: row ceiling %d is not from 1 to %d", ErrInvalidKey, q.MaxRows, MaxWaitingRows)
	}
	if q.Names < AnyName || q.Names > NamedOnly {
		return fmt.Errorf("%w: name filter %d", ErrInvalidKey, q.Names)
	}
	if len(q.States) == 0 {
		return fmt.Errorf("%w: no channel type to list waiting sessions of", ErrInvalidKey)
	}
	for typ, states := range q.States {
		if !ValidName(typ) {
			return fmt.Errorf("%w: channel type %q", ErrInvalidKey, typ)
		}
		if len(states) == 0 {
			return fmt.Errorf("%w: channel type %s lists no session state", ErrInvalidKey, typ)
		}
		for _, st := range states {
			if st != "" && !ValidStateName(st) {
				return fmt.Errorf("%w: session state %q", ErrInvalidKey, st)
			}
		}
	}
	if len(q.Anchors) == 0 && !q.CreatorBranch() {
		return fmt.Errorf("%w: neither anchors nor a creator to list waiting sessions of", ErrInvalidKey)
	}
	if len(q.Anchors) > MaxWaitingAnchors {
		return fmt.Errorf("%w: %d anchors, over %d", ErrInvalidKey, len(q.Anchors), MaxWaitingAnchors)
	}
	for _, a := range q.Anchors {
		if err := a.Validate(); err != nil {
			return err
		}
	}
	for _, typ := range q.CreatorTypes {
		if !ValidName(typ) {
			return fmt.Errorf("%w: channel type %q", ErrInvalidKey, typ)
		}
	}
	return nil
}

// TooManyWaiting is the error of a [Store.ListWaiting] call that matched
// more than [WaitingQuery.RowCeiling] distinct channels.
func TooManyWaiting(q WaitingQuery) error {
	return fmt.Errorf("%w: more than %d waiting channels", ErrTooManyRows, q.RowCeiling())
}
