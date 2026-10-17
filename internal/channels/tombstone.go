package channels

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TombstoneHandle names exactly one batch of channels that one
// [Store.Tombstone] call moved out of reach, so [Store.Untombstone] can put
// back that batch and no other. Two tombstones of one id are two batches
// with two handles.
//
// It is opaque to callers: only a store makes one. [TombstoneHandle.String]
// and [ParseTombstoneHandle] round-trip it, for a caller that must keep it
// past the process. The zero handle names no batch: a tombstone that found
// nothing to move returns it, and restoring it is a no-op.
type TombstoneHandle struct {
	anchor Anchor
	stamp  string
}

// stampPattern is the grammar of [TombstoneStamp]. A handle's stamp names a
// file path on the file backend, so a parsed one must match it exactly.
var stampPattern = regexp.MustCompile(`^\d{8}T\d{6}\.\d{6}Z-[0-9a-z]{6}$`)

// handlePrefix versions the text form of a handle.
const handlePrefix = "t1"

var handleEnc = base64.RawURLEncoding

// NewTombstoneHandle returns the handle of the batch a store filed under
// stamp for anchor. Backends call it; nothing else needs to.
func NewTombstoneHandle(anchor Anchor, stamp string) TombstoneHandle {
	return TombstoneHandle{anchor: anchor, stamp: stamp}
}

// IsZero reports whether h names no batch.
func (h TombstoneHandle) IsZero() bool { return h.stamp == "" }

// Anchor returns the anchor the batch was taken from: the entity id, and
// the face for a face tombstone or an empty face for the whole entity.
func (h TombstoneHandle) Anchor() Anchor { return h.anchor }

// Stamp returns the label the batch is filed under (see [TombstoneStamp]).
func (h TombstoneHandle) Stamp() string { return h.stamp }

// Validate reports whether h is a well-formed, non-zero handle.
func (h TombstoneHandle) Validate() error {
	if h.IsZero() {
		return fmt.Errorf("%w: empty tombstone handle", ErrInvalidKey)
	}
	if err := h.anchor.Validate(); err != nil {
		return err
	}
	if !stampPattern.MatchString(h.stamp) {
		return fmt.Errorf("%w: tombstone stamp %q", ErrInvalidKey, h.stamp)
	}
	return nil
}

// String renders h for [ParseTombstoneHandle]. The zero handle renders as
// the empty string.
func (h TombstoneHandle) String() string {
	if h.IsZero() {
		return ""
	}
	return strings.Join([]string{
		handlePrefix,
		handleEnc.EncodeToString([]byte(h.anchor.ID)),
		handleEnc.EncodeToString([]byte(h.anchor.Face)),
		h.stamp,
	}, ":")
}

// ParseTombstoneHandle reads a handle written by [TombstoneHandle.String].
// The empty string is the zero handle. Anything else that is not a valid
// handle is [ErrInvalidKey].
func ParseTombstoneHandle(s string) (TombstoneHandle, error) {
	if s == "" {
		return TombstoneHandle{}, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) != 4 || parts[0] != handlePrefix {
		return TombstoneHandle{}, fmt.Errorf("%w: malformed tombstone handle", ErrInvalidKey)
	}
	id, err := handleEnc.DecodeString(parts[1])
	if err != nil {
		return TombstoneHandle{}, fmt.Errorf("%w: malformed tombstone handle", ErrInvalidKey)
	}
	face, err := handleEnc.DecodeString(parts[2])
	if err != nil {
		return TombstoneHandle{}, fmt.Errorf("%w: malformed tombstone handle", ErrInvalidKey)
	}
	h := TombstoneHandle{anchor: Anchor{ID: string(id), Face: entity.Face(face)}, stamp: parts[3]}
	if err := h.Validate(); err != nil {
		return TombstoneHandle{}, err
	}
	return h, nil
}

// OccupiedError reports an [Store.Untombstone] refused because live
// channels now sit where the batch would go back: on the entity id for a
// whole-entity batch, on the face for a face batch. Nothing was restored;
// the batch stays tombstoned.
type OccupiedError struct {
	Anchor Anchor
}

func (e *OccupiedError) Error() string {
	return fmt.Sprintf("channels: cannot restore the tombstoned channels of %s: live channels occupy it",
		entity.FormatStateRef(e.Anchor.ID, e.Anchor.Face))
}
