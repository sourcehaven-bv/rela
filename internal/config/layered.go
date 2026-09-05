package config

import (
	"context"
	"errors"
	"maps"
	"os"
	"slices"
)

// layered reads from primary, falling back to secondary when primary does
// not have the file.
type layered struct {
	primary   Loader
	secondary Loader
}

var _ Loader = (*layered)(nil)

// NewLayered returns a Loader that serves each name from primary when
// present and from secondary otherwise.
//
// It exists so config baked into a database can back a project whose files
// still live on disk, with DISK FIRST: a project that has both is a project
// being edited, and the file the operator just wrote must win over the copy
// baked in at package time. That ordering is also what makes the migration
// to a database-backed project safe to do one consumer at a time — with the
// filesystem in front, a converted call site behaves exactly as it did
// before until the file is actually removed.
//
// The fall-through rule is [Loader.Load]'s; see [layered.Load].
//
// Nil: both loaders are rejected when nil. A layered loader is built at a
// wiring site that already threads errors, so a missing collaborator becomes
// an actionable startup failure rather than a panic at the first read.
func NewLayered(primary, secondary Loader) (Loader, error) {
	if primary == nil || secondary == nil {
		return nil, errors.New("config: NewLayered requires two non-nil loaders")
	}
	return &layered{primary: primary, secondary: secondary}, nil
}

// Load returns the primary's bytes, or the secondary's when the primary
// does not have the file.
//
// Only os.ErrNotExist falls through. Any other primary error surfaces
// unchanged, because a permission error or a truncated read is not evidence
// that the file is absent, and treating it as such would silently serve
// stale baked config in place of the config the operator is looking at.
func (l *layered) Load(ctx context.Context, name string) ([]byte, error) {
	data, err := l.primary.Load(ctx, name)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return l.secondary.Load(ctx, name)
}

// List returns the UNION of both layers, deduplicated and sorted.
//
// A union rather than [layered.Load]'s precedence rule, because the two
// answer different questions. Load asks "which of these two files do I
// read", and exactly one must win. List asks "what is in this directory",
// and a script present only in the database is genuinely there — shadowing
// the whole directory because one file of the same name sits on disk would
// make it invisible. Per-file precedence is preserved anyway: a name in both
// layers appears once, and the subsequent Load resolves it to the primary's
// bytes.
//
// An error from EITHER layer fails the whole call; there is no partial
// union. A half-listed directory is the worse failure: a caller cannot tell
// it from a complete one, so an automation missing because the database
// hiccuped would look exactly like an automation that was never configured.
// Failing loudly keeps that diagnosable.
func (l *layered) List(ctx context.Context, dir string) ([]string, error) {
	primary, err := l.primary.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	secondary, err := l.secondary.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	// Built by copying into a fresh map rather than appending secondary onto
	// primary: primary's backing array belongs to the layer that returned it,
	// and appending into its spare capacity would write through to a slice
	// that layer may still be holding.
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	for _, n := range primary {
		seen[n] = struct{}{}
	}
	for _, n := range secondary {
		seen[n] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// Subscribe forwards to whichever layer can watch for changes, primary
// first, and reports no capability when neither can.
//
// Without this, wrapping an FSLoader would silently disable live config
// reload: [Subscriber] is an optional interface consumers type-assert for,
// so the decorator would fail the assertion with no compile error and no
// test failure — an operator editing data-entry.yaml would simply see
// nothing happen. That directly undercuts the disk-first premise, which
// exists so an operator's edit is the one that takes effect.
func (l *layered) Subscribe(ctx context.Context, name string, onChange func()) (func(), error) {
	for _, candidate := range []Loader{l.primary, l.secondary} {
		if sub, ok := candidate.(Subscriber); ok {
			return sub.Subscribe(ctx, name, onChange)
		}
	}
	return nil, errors.New("config: no layer supports change notification")
}
