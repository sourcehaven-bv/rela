package metamodel

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PileDecl is one entry of the top-level `piles:` block: a pile every user
// holds, such as favorites or notifications (internal/piles). The map key is
// the pile's key, which toggles, navigation entries and `notify` actions name.
//
// It declares policy only. A pile's items are per-user state outside the
// graph, so nothing here is an entity type.
type PileDecl struct {
	// Name is the label users see.
	Name string `yaml:"name"`
	// Icon is a pile icon; the piles service checks it against its allowlist.
	Icon string `yaml:"icon,omitempty"`
	// OnRepeat says what an add of a ref already on the pile does: "keep"
	// (the default) leaves it in place, "bump" moves it to the top, unread.
	OnRepeat string `yaml:"on_repeat,omitempty"`
	// ReadState lets users mark items read.
	ReadState bool `yaml:"read_state,omitempty"`
	// AutoRead is "off" (the default) or "on_open": an item is marked read
	// when its owner opens the entity.
	AutoRead string `yaml:"auto_read,omitempty"`
	// ClearReadAfter drops items this long after they were read, such as
	// "12h" or "7d". Empty keeps them.
	ClearReadAfter string `yaml:"clear_read_after,omitempty"`
	// ExpireAfter drops items this long after they were added. Empty keeps
	// them.
	ExpireAfter string `yaml:"expire_after,omitempty"`
	// Reasons maps the reason keys a `notify` action may record to the label
	// users see.
	Reasons map[string]string `yaml:"reasons,omitempty"`
}

// Values of [PileDecl.OnRepeat] and [PileDecl.AutoRead].
const (
	PileOnRepeatKeep = "keep"
	PileOnRepeatBump = "bump"
	PileAutoReadOff  = "off"
	PileAutoReadOpen = "on_open"
)

// pileKeyPattern is the shape of a declared pile key and of a reason key.
var pileKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// Bump reports whether a repeat add moves the item to the top.
func (d PileDecl) Bump() bool { return d.OnRepeat == PileOnRepeatBump }

// ReadsOnOpen reports whether opening an entity marks it read.
func (d PileDecl) ReadsOnOpen() bool { return d.AutoRead == PileAutoReadOpen }

// ClearReadDuration is ClearReadAfter parsed; zero when unset. Load
// validation has refused a value that does not parse.
func (d PileDecl) ClearReadDuration() time.Duration {
	v, _ := ParsePileDuration(d.ClearReadAfter)
	return v
}

// ExpireDuration is ExpireAfter parsed; zero when unset.
func (d PileDecl) ExpireDuration() time.Duration {
	v, _ := ParsePileDuration(d.ExpireAfter)
	return v
}

// PileKeys returns the declared pile keys, sorted.
func (m *Metamodel) PileKeys() []string {
	keys := make([]string, 0, len(m.Piles))
	for k := range m.Piles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pileDurationUnits are the units [ParsePileDuration] adds to Go's.
var pileDurationUnits = map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour}

// ParsePileDuration parses a pile expiry: a Go duration such as "30m" or
// "12h", or a whole number of days or weeks such as "7d" or "2w". Empty is
// zero.
func ParsePileDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	for unit, size := range pileDurationUnits {
		if num, ok := strings.CutSuffix(s, unit); ok {
			n, err := strconv.Atoi(num)
			if err != nil || n <= 0 {
				return 0, fmt.Errorf("%q is not a duration such as 30m, 12h, 7d or 2w", s)
			}
			return time.Duration(n) * size, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%q is not a duration such as 30m, 12h, 7d or 2w", s)
	}
	return d, nil
}

// validatePiles checks the top-level `piles:` block.
func validatePiles(m *Metamodel) []string {
	var errs []string
	for _, key := range m.PileKeys() {
		d := m.Piles[key]
		at := "piles." + key
		if !pileKeyPattern.MatchString(key) {
			errs = append(errs, fmt.Sprintf("%s: a pile key is lowercase letters, digits, - and _, "+
				"starting with a letter", at))
		}
		if strings.TrimSpace(d.Name) == "" {
			errs = append(errs, at+": name is required")
		}
		switch d.OnRepeat {
		case "", PileOnRepeatKeep, PileOnRepeatBump:
		default:
			errs = append(errs, fmt.Sprintf("%s: on_repeat must be %q or %q", at, PileOnRepeatKeep, PileOnRepeatBump))
		}
		switch d.AutoRead {
		case "", PileAutoReadOff:
		case PileAutoReadOpen:
			if !d.ReadState {
				errs = append(errs, at+": auto_read needs read_state: true")
			}
		default:
			errs = append(errs, fmt.Sprintf("%s: auto_read must be %q or %q", at, PileAutoReadOff, PileAutoReadOpen))
		}
		if _, err := ParsePileDuration(d.ClearReadAfter); err != nil {
			errs = append(errs, fmt.Sprintf("%s: clear_read_after: %v", at, err))
		} else if d.ClearReadAfter != "" && !d.ReadState {
			errs = append(errs, at+": clear_read_after needs read_state: true")
		}
		if _, err := ParsePileDuration(d.ExpireAfter); err != nil {
			errs = append(errs, fmt.Sprintf("%s: expire_after: %v", at, err))
		}
		reasons := make([]string, 0, len(d.Reasons))
		for r := range d.Reasons {
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		for _, r := range reasons {
			if !pileKeyPattern.MatchString(r) {
				errs = append(errs, fmt.Sprintf("%s: reason key %q must be lowercase letters, digits, - and _", at, r))
			}
			if strings.TrimSpace(d.Reasons[r]) == "" {
				errs = append(errs, fmt.Sprintf("%s: reason %q needs a label", at, r))
			}
		}
	}
	return errs
}
