package syncmerge

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// State is one side: properties, the body, and the names its reader
// withheld.
type State struct {
	Properties map[string]any
	Content    string
	Redacted   []string
}

// Theirs is the other system's state. A property key that is absent was not
// reported; [Empty] clears. Content nil means not reported.
type Theirs struct {
	Properties map[string]any
	Content    *string
}

// Conflict is one field both sides changed.
type Conflict struct {
	Field              string
	Base, Ours, Theirs any
}

// Result is the classification.
type Result struct {
	// BaseUnknown: there was no base, so nothing is written or pushed.
	BaseUnknown bool
	// Write holds canonical theirs values to write into rela; nil clears.
	Write map[string]any
	// WriteContent is the body to write, or nil.
	WriteContent *string
	// Push holds ours values to send to the other system. The body, when
	// pushed, is under [ContentField].
	Push      map[string]any
	Conflicts []Conflict
	Unchanged []string
	// Complete: theirs reported every synced field. A field theirs did not
	// report was not compared, so a local edit to it may still be unpushed.
	Complete bool
	// Retag: the report was [Result.Complete], nothing is to be written or
	// pushed, there are no conflicts, and the base is unknown or differs
	// from ours on a field both sides agree on. Both sides converged, so the
	// caller tags the state it read as the new base.
	Retag bool
}

// Merge classifies fields. base may be nil. A field withheld from ours' or
// base's reader is an error: merging against a hidden value would write or
// push a guess.
func Merge(fields []Field, base *State, ours State, theirs Theirs) (Result, error) {
	res := Result{Write: map[string]any{}, Push: map[string]any{}, BaseUnknown: base == nil, Complete: true}
	baseDiffers := false
	for _, f := range fields {
		if slices.Contains(ours.Redacted, f.Name) || (base != nil && slices.Contains(base.Redacted, f.Name)) {
			return Result{}, fmt.Errorf("%w: field %q is hidden from this reader", ErrField, f.Name)
		}
		o := valueOf(f, ours)
		var b any
		if base != nil {
			b = valueOf(f, *base)
		}
		t, reported, err := theirsValue(f, theirs)
		if err != nil {
			return Result{}, err
		}
		if !reported {
			res.Complete = false
			continue
		}
		switch {
		case Equal(f, o, t):
			res.Unchanged = append(res.Unchanged, f.Name)
			if base == nil || !Equal(f, b, o) {
				baseDiffers = true
			}
		case base == nil:
			res.Conflicts = append(res.Conflicts, Conflict{Field: f.Name, Ours: o, Theirs: t})
		case Equal(f, b, o):
			if f.Kind == KindContent {
				s, _ := t.(string)
				res.WriteContent = &s
			} else {
				res.Write[f.Name] = t
			}
		case Equal(f, b, t):
			res.Push[f.Name] = o
		default:
			res.Conflicts = append(res.Conflicts, Conflict{Field: f.Name, Base: b, Ours: o, Theirs: t})
		}
	}
	res.Retag = res.Complete && len(res.Conflicts) == 0 && len(res.Write) == 0 && res.WriteContent == nil &&
		len(res.Push) == 0 && baseDiffers
	sort.Strings(res.Unchanged)
	return res, nil
}

func valueOf(f Field, s State) any {
	if f.Kind == KindContent {
		return s.Content
	}
	return s.Properties[f.Name]
}

// theirsValue returns the canonical theirs value of f and whether it was
// reported. [Empty] canonicalizes to nil.
func theirsValue(f Field, th Theirs) (value any, reported bool, err error) {
	if f.Kind == KindContent {
		if th.Content == nil {
			return nil, false, nil
		}
		return normalizeContent(*th.Content), true, nil
	}
	v, ok := th.Properties[f.Name]
	if !ok {
		return nil, false, nil
	}
	if v == Empty || metamodel.IsEmptyValue(v) {
		return nil, true, nil
	}
	c, err := Canonical(f, v)
	if err != nil {
		return nil, true, err
	}
	return c, true, nil
}
