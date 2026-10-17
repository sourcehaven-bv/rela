package channels

import (
	"fmt"
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
type Filing struct {
	Policy TombstonePolicy
	// PurgeAfter is the earliest time the batch may be purged. Zero means
	// not recorded.
	PurgeAfter time.Time
	Pending    bool
}

// Recorded reports whether f carries a purge time.
func (f Filing) Recorded() bool { return !f.PurgeAfter.IsZero() }

// PrepareFiling validates f and returns it with its time normalized.
func PrepareFiling(f Filing) (Filing, error) {
	if !f.Policy.Valid() {
		return f, fmt.Errorf("%w: tombstone policy %q", ErrInvalidMessage, f.Policy)
	}
	f.PurgeAfter = NormTime(f.PurgeAfter)
	return f, nil
}

// CheckPurge validates the author and time of a [Store.Purge] call.
func CheckPurge(by Author, at time.Time) error {
	if err := checkAuthor(by); err != nil {
		return err
	}
	if at.IsZero() {
		return fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	return nil
}

// PurgeAudit returns the record [Store.Purge] appends to the batch's filed
// trail.
func PurgeAudit(h TombstoneHandle, by Author, at time.Time) AuditRecord {
	return AuditRecord{At: NormTime(at), Op: AuditPurge, Face: h.Anchor().Face, By: by}
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
