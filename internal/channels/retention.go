package channels

import (
	"fmt"
	"slices"
	"time"
)

// TombstonePolicy says why a batch was tombstoned, and so which retention
// applies to it.
type TombstonePolicy string

// Tombstone policies. The empty policy is a batch filed without one: by a
// release before retention existed, or by a caller that records none. Its
// purge time is decided from the configuration when the sweep runs.
const (
	PolicyUnrecorded TombstonePolicy = ""
	// PolicyDelete is a batch of an entity deleted under
	// `on_entity_delete: delete`.
	PolicyDelete TombstonePolicy = "delete"
	// PolicyDetach is a batch kept for a while after its entity went: an
	// entity deleted under `detach`, or a leftover moved aside for a new
	// entity with the same id.
	PolicyDetach TombstonePolicy = "detach"
	// PolicyOrphan is a batch the retention sweep moved aside because its
	// entity was gone without a delete through rela.
	PolicyOrphan TombstonePolicy = "orphan"
)

// Valid reports whether p is one of the declared policies.
func (p TombstonePolicy) Valid() bool {
	switch p {
	case PolicyUnrecorded, PolicyDelete, PolicyDetach, PolicyOrphan:
		return true
	}
	return false
}

// Filing is what a batch records when it is tombstoned, so the purge later
// follows the rules that held when the entity went rather than the
// configuration at purge time.
//
// Pending marks a batch tombstoned before the delete that caused it has
// committed. [Store.Purge] refuses a pending batch: the delete may still
// fail, and then the batch goes back. [Store.ConfirmTombstone] clears the
// mark once the delete has committed. The zero Filing is a confirmed batch
// with nothing recorded.
//
// A batch is restorable while it is pending, while its policy is
// [PolicyOrphan], or when it recorded no policy; see [Filing.Restorable].
// Confirming a batch makes it final: [Filing.Confirmed].
type Filing struct {
	Policy TombstonePolicy
	// PurgeAfter is the earliest time the whole batch may be purged. Zero
	// means not recorded.
	PurgeAfter time.Time
	// TypeAfter holds the channel types of the batch that may be purged
	// before PurgeAfter, each with the earliest time it may be. A delete
	// that erases one type at once and keeps another for its retention
	// files both in one batch this way. Nil when every type waits for
	// PurgeAfter.
	TypeAfter map[string]time.Time
	Pending   bool
}

// Recorded reports whether f carries a purge time.
func (f Filing) Recorded() bool { return !f.PurgeAfter.IsZero() }

// DueAt returns the earliest time anything in the batch may be purged:
// PurgeAfter, or an earlier time in TypeAfter. Zero when not recorded.
func (f Filing) DueAt() time.Time {
	due := f.PurgeAfter
	for _, t := range f.TypeAfter {
		if t.Before(due) {
			due = t
		}
	}
	return due
}

// Restorable reports whether [Store.Untombstone] may put the batch back:
// it is pending, an orphan, or recorded no policy. A confirmed delete or
// detach batch is final, so an id reused by a new entity never receives
// the channels of the old one.
func (f Filing) Restorable() bool {
	return f.Pending || f.Policy == PolicyOrphan || f.Policy == PolicyUnrecorded
}

// Confirmed returns f as [Store.ConfirmTombstone] leaves it: not pending,
// and an orphan batch turned into a detach batch, so neither can be
// restored.
func (f Filing) Confirmed() Filing {
	f.Pending = false
	if f.Policy == PolicyOrphan {
		f.Policy = PolicyDetach
	}
	return f
}

// Filer computes the filing of a batch from the channels the batch moves.
// [Store.Tombstone] calls it once, under the same lock or transaction as
// the move. So a channel created after the caller listed the entity is
// counted, and cannot be purged sooner than its own type allows. An error
// aborts the tombstone: nothing moves, and [Store.Tombstone] returns the
// error as it is.
//
// Nil: accepted by [Store.Tombstone], which then files the zero Filing.
type Filer func(moved []Channel) (Filing, error)

// FileAs returns a Filer that files f whatever the batch holds.
func FileAs(f Filing) Filer { return func([]Channel) (Filing, error) { return f, nil } }

// FileBatch runs file over moved and validates the result; see
// [PrepareFiling]. Backends call it inside the Tombstone critical section.
//
// Nil: file is accepted and gives the zero Filing.
func FileBatch(file Filer, moved []Channel) (Filing, error) {
	if file == nil {
		return Filing{}, nil
	}
	f, err := file(moved)
	if err != nil {
		return Filing{}, err
	}
	return PrepareFiling(f)
}

// PrepareFiling validates f and returns it with its times normalized. A
// TypeAfter entry at or after PurgeAfter says nothing and is dropped; one
// without a PurgeAfter, or with an invalid type name, is an error.
func PrepareFiling(f Filing) (Filing, error) {
	if !f.Policy.Valid() {
		return f, fmt.Errorf("%w: tombstone policy %q", ErrInvalidMessage, f.Policy)
	}
	f.PurgeAfter = NormTime(f.PurgeAfter)
	if len(f.TypeAfter) == 0 {
		f.TypeAfter = nil
		return f, nil
	}
	if f.PurgeAfter.IsZero() {
		return f, fmt.Errorf("%w: type purge times without a batch purge time", ErrInvalidMessage)
	}
	early := make(map[string]time.Time, len(f.TypeAfter))
	for typ, t := range f.TypeAfter {
		if !ValidName(typ) {
			return f, fmt.Errorf("%w: channel type %q", ErrInvalidKey, typ)
		}
		if t = NormTime(t); !t.IsZero() && t.Before(f.PurgeAfter) {
			early[typ] = t
		}
	}
	f.TypeAfter = nil
	if len(early) > 0 {
		f.TypeAfter = early
	}
	return f, nil
}

// CheckPurge validates the types, author and time of a [Store.Purge] call.
func CheckPurge(types []string, by Author, at time.Time) error {
	for _, typ := range types {
		if !ValidName(typ) {
			return fmt.Errorf("%w: channel type %q", ErrInvalidKey, typ)
		}
	}
	if err := checkAuthor(by); err != nil {
		return err
	}
	if at.IsZero() {
		return fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	return nil
}

// PurgeAudit returns the records [Store.Purge] appends to the batch's
// filed trail: one per erased channel, naming it, or one naming only the
// anchor when the batch held no channel. Each carries the batch's stamp.
func PurgeAudit(h TombstoneHandle, erased []Channel, by Author, at time.Time) []AuditRecord {
	if len(erased) == 0 {
		return []AuditRecord{{At: NormTime(at), Op: AuditPurge, Face: h.Anchor().Face, By: by, Tomb: h.Stamp()}}
	}
	out := make([]AuditRecord, len(erased))
	for i, c := range erased {
		out[i] = KeyAudit(c.Key, AuditPurge, NormTime(at))
		out[i].By, out[i].Tomb, out[i].Seq = by, h.Stamp(), c.LastSeq
	}
	return out
}

// PurgeTypes returns the channels of chs whose type is in types. An
// empty types means every channel.
func PurgeTypes(chs []Channel, types []string) []Channel {
	if len(types) == 0 {
		return chs
	}
	out := []Channel{}
	for _, c := range chs {
		if slices.Contains(types, c.Key.Type) {
			out = append(out, c)
		}
	}
	return out
}

// CheckDropType validates the type a [Store.DropTypeRecord] call names.
func CheckDropType(typ string) error {
	if !ValidName(typ) {
		return fmt.Errorf("%w: channel type %q", ErrInvalidKey, typ)
	}
	return nil
}

// TombstoneBatch is one tombstoned batch as [Store.Tombstones] lists it:
// its handle, when it was filed, what it recorded, and the metadata of its
// channels.
type TombstoneBatch struct {
	Handle   TombstoneHandle
	At       time.Time
	Filing   Filing
	Channels []Channel
}

// stampTimeLayout is the time part of a [TombstoneStamp].
const stampTimeLayout = "20060102T150405.000000Z"

// StampTime returns the time a [TombstoneStamp] was made at.
func StampTime(stamp string) (time.Time, error) {
	if !stampPattern.MatchString(stamp) {
		return time.Time{}, fmt.Errorf("%w: tombstone stamp %q", ErrInvalidKey, stamp)
	}
	t, err := time.Parse(stampTimeLayout, stamp[:len(stampTimeLayout)])
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: tombstone stamp %q: %w", ErrInvalidKey, stamp, err)
	}
	return t.UTC(), nil
}

// StampBefore returns the lowest stamp made at t or later. Stamps order as
// their times do, so a stamp is below the result exactly when it was made
// before t.
func StampBefore(t time.Time) string { return NormTime(t).Format(stampTimeLayout) }

// PurgedTrail is the label the filed audit trail of a purged batch is kept
// under. A purge moves the trail there, so neither Untombstone nor a second
// Purge finds the batch again, while the trail itself survives.
func PurgedTrail(stamp string) string { return "purged-" + stamp }
