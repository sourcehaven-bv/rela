package store

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// EntityVersion is an opaque compare-and-swap token for one stored entity.
//
// It is derived from the stored record's own bytes — id, type, sorted
// properties, body — and NOT from a counter the store maintains. Two
// consequences follow, both deliberate:
//
//   - Every mutation route changes it, including ones the store never
//     performed. fsstore reconciles hand edits and `git pull`; a counter
//     column is bumped only by a write the store itself made, so it would
//     report "unchanged" for a file that demonstrably changed on disk. A
//     CAS that silently passes is worse than no CAS at all.
//   - An A->B->A round trip is INVISIBLE to the compare. That is the safe
//     direction: the caller's write is being applied against a state
//     byte-identical to the one it read, so applying it is exactly right.
//     (A counter would reject it, which is stricter but not more correct.)
//
// The token deliberately EXCLUDES relations, unlike the data-entry API's
// ETag. [EntityWriter.UpdateEntity] can neither read nor write relations, so
// a token sensitive to them would produce conflicts that retrying the guarded
// write can never clear — the retry recomputes the same entity write and loses
// to the same unrelated edge every time. A CAS must guard exactly what the
// guarded operation can change.
//
// Callers MUST treat the value as opaque: read it from [VersionOf] (or from a
// read that returned one) and hand it back unmodified. The derivation is not
// part of the contract and may change; what is guaranteed is that the token
// for a given stored state is stable, and differs when that state differs.
type EntityVersion string

// UpdateCondition is the compare-and-swap precondition on a conditional
// write. The zero value means "no condition" — an unconditional write, which
// is what plain [EntityWriter.UpdateEntity] performs.
//
// This is a struct rather than a bare EntityVersion so that "no expectation"
// and "expect the empty version" cannot be confused, and so a future
// precondition (expect-absent, expect-any-of) can be added without changing
// every backend's signature again.
type UpdateCondition struct {
	// ExpectedVersion is the token the caller last observed. The write
	// applies only if the stored record still hashes to it; otherwise the
	// backend returns a *VersionConflictError and writes nothing.
	ExpectedVersion EntityVersion
}

// IsZero reports whether the condition imposes no precondition at all.
func (c UpdateCondition) IsZero() bool { return c.ExpectedVersion == "" }

// VersionConflictError is returned by a conditional write whose precondition
// did not hold: the stored record changed since the caller read it. NOTHING
// was written.
//
// Retry ergonomics are the whole point, so the shape is load-bearing:
//
//   - Match it with errors.As to recover Expected/Actual for logging, or with
//     errors.Is(err, ErrConflict) if you only care that the write lost a race.
//   - Actual is the CURRENT stored version, so a caller can re-read, recompute
//     its intended change against the new state, and retry with Actual as its
//     new ExpectedVersion. That loop is bounded by the caller, not the store.
//
// **Every layer that re-presents this error MUST wrap it with %w.** See
// RR-HI9QIU: a translation that built a fresh error with no wrapped cause made
// errors.As fail silently, which turned a retry loop into dead code — measured
// as 8/8 concurrent losers receiving a 500 instead of retrying. A conflict
// error that cannot be recognized is indistinguishable from a bug.
type VersionConflictError struct {
	ID       string
	Expected EntityVersion
	Actual   EntityVersion
}

func (e *VersionConflictError) Error() string {
	return fmt.Sprintf(
		"store: version conflict on %s: expected %s, stored %s",
		e.ID, e.Expected, e.Actual,
	)
}

// Is reports a version conflict as an ErrConflict, so callers that only need
// "this write lost a race" keep working with errors.Is while callers that want
// to retry can errors.As for the versions.
func (e *VersionConflictError) Is(target error) bool {
	return errors.Is(target, ErrConflict)
}

// VersionOf computes the compare-and-swap token for an entity's state.
//
// It is the single derivation every backend uses, so a token minted by one
// backend means the same thing as one minted by another and no backend can
// drift into its own hashing. Nil returns the empty version.
//
// Fields NOT folded in, each for a reason:
//
//   - UpdatedAt: a wall-clock stamp the store sets on write. Including it
//     would make the token change on a no-op re-save and, worse, differ
//     between two backends holding identical content.
//   - Redacted / Inaccessible: per-reader artifacts, not stored content. A
//     redacted read must produce the SAME token as an unredacted one, or a
//     principal who cannot see every field could never satisfy a CAS.
//   - Relations: see [EntityVersion].
func VersionOf(e *entity.Entity) EntityVersion {
	if e == nil {
		return ""
	}
	h := sha256.New()
	// Length-prefix every field so ("ab","c") and ("a","bc") cannot collide.
	writeVersionField(h, e.ID)
	writeVersionField(h, e.Type)
	writeVersionField(h, e.Content)

	keys := make([]string, 0, len(e.Properties))
	for k := range e.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := e.Properties[k]
		writeVersionField(h, k)
		// Fold the TYPE in alongside the rendering. %v alone is type-blind:
		// int64(1), float64(1) and the string "1" all render as "1", as do
		// true and "true", and []string{"a","b"} and "[a b]". Two entities
		// differing only in a property's type would then share a token, so a
		// CAS that ought to conflict would silently succeed and the other
		// writer's change would be lost — the exact failure this token exists
		// to prevent. There is no coercion layer normalising property types on
		// the write path, so a script writing 1 where a form wrote "1" is an
		// ordinary occurrence rather than a contrived one.
		writeVersionField(h, fmt.Sprintf("%T", v))
		writeVersionField(h, fmt.Sprintf("%v", v))
	}
	return EntityVersion(base64.RawURLEncoding.EncodeToString(h.Sum(nil)))
}

// writeVersionField hashes one length-prefixed field. The prefix is what makes
// the concatenation unambiguous, so a property value can never be crafted to
// look like the start of the next field.
func writeVersionField(h io.Writer, s string) {
	_, _ = fmt.Fprintf(h, "%d:", len(s))
	_, _ = io.WriteString(h, s)
}
