package piles

import (
	"fmt"
	"regexp"
	"slices"
	"time"
)

// keyPattern is the shape of a declared pile's key.
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ValidKey reports whether key has the shape of a declared pile's key.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Declared is a pile the schema declares under the top-level `piles:` key.
// Every user holds one, made on first use. Its name and icon come from the
// schema, and users cannot rename or delete it ([ErrLocked]).
type Declared struct {
	// Key identifies the pile in the schema and on the wire.
	Key  string
	Name string
	Icon string
	// Bump moves an item that is already on the pile back to the top, unread,
	// with the new actor and reason. Without it a repeat add is a no-op.
	Bump bool
	// ReadState lets the user mark items read.
	ReadState bool
	// AutoRead marks an item read when its owner opens the entity.
	AutoRead bool
	// ClearReadAfter drops items this long after they were read.
	ClearReadAfter time.Duration
	// ExpireAfter drops items this long after they were added.
	ExpireAfter time.Duration
	// Reasons maps a reason key to its label.
	Reasons map[string]string
}

func (d Declared) validate() error {
	if !ValidKey(d.Key) {
		return fmt.Errorf("%w: declared pile key %q must match %s", ErrInvalid, d.Key, keyPattern)
	}
	if _, err := cleanName(d.Name); err != nil {
		return fmt.Errorf("declared pile %q: %w", d.Key, err)
	}
	if !slices.Contains(Icons, d.Icon) {
		return fmt.Errorf("%w: declared pile %q has unknown icon %q", ErrInvalid, d.Key, d.Icon)
	}
	if d.AutoRead && !d.ReadState {
		return fmt.Errorf("%w: declared pile %q reads on open but keeps no read state", ErrInvalid, d.Key)
	}
	if d.ClearReadAfter < 0 || d.ExpireAfter < 0 {
		return fmt.Errorf("%w: declared pile %q has a negative expiry", ErrInvalid, d.Key)
	}
	if d.ClearReadAfter > 0 && !d.ReadState {
		return fmt.Errorf("%w: declared pile %q clears read items but keeps no read state", ErrInvalid, d.Key)
	}
	return nil
}

// cutoffs returns the read and added times before which an item has expired
// at now. A zero time means that expiry is off.
func (d Declared) cutoffs(now time.Time) (readBefore, addedBefore time.Time) {
	if d.ClearReadAfter > 0 {
		readBefore = now.Add(-d.ClearReadAfter)
	}
	if d.ExpireAfter > 0 {
		addedBefore = now.Add(-d.ExpireAfter)
	}
	return readBefore, addedBefore
}
