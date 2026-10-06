package desktopnotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sync"
)

// stateKeyPrefix namespaces the tracker's keys in the state store. The
// "rule-" prefix keeps a rule id such as "con" from forming a Windows
// reserved name, which state keys may not contain.
const stateKeyPrefix = "desktop-notify/rule-"

// KV is the part of state.KV the tracker uses. A missing key is an error
// satisfying errors.Is(err, fs.ErrNotExist).
type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
}

// Tracker remembers which matches have been notified, per rule, and reports
// only the new ones. It is safe for concurrent use; calls are serialized.
//
// Two trackers on the same KV do not coordinate. The desktop app runs one.
type Tracker struct {
	kv KV
	mu sync.Mutex
}

// trackerState is the JSON stored per rule.
type trackerState struct {
	// Seen holds the ids of the entities matching at the last update,
	// sorted.
	Seen []string `json:"seen"`
}

// NewTracker returns a Tracker persisting to kv.
func NewTracker(kv KV) (*Tracker, error) {
	if kv == nil {
		return nil, errors.New("desktopnotify: NewTracker requires a state KV")
	}
	return &Tracker{kv: kv}, nil
}

// Update records res and returns the matches that were not matching at the
// previous update. Those are the notifications to show.
//
// For a rule with no stored record (first launch, or a newly added rule), it
// records the current matches and returns none of them; see the package
// documentation. An entity that no longer matches is dropped from the record,
// so it is reported again if it matches later.
//
// State for a rule that is no longer in the config is left in place. It is
// small, and keeping it means removing and restoring a rule does not
// re-notify.
//
// On error the returned matches are those already recorded as notified, so
// the caller should still show them. Rules not yet recorded are retried on
// the next update.
func (t *Tracker) Update(ctx context.Context, res Result) ([]Match, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	byRule := make(map[string][]Match, len(res.Rules))
	for _, m := range res.Matches {
		byRule[m.RuleID] = append(byRule[m.RuleID], m)
	}

	var fresh []Match
	for _, ruleID := range res.Rules {
		matches := byRule[ruleID]
		prev, known, err := t.load(ctx, ruleID)
		if err != nil {
			return fresh, err
		}
		current := make([]string, 0, len(matches))
		var added []Match
		for _, m := range matches {
			current = append(current, m.EntityID)
			if _, found := slices.BinarySearch(prev, m.EntityID); known && !found {
				added = append(added, m)
			}
		}
		slices.Sort(current)
		current = slices.Compact(current)
		if !known || !slices.Equal(prev, current) {
			if err := t.save(ctx, ruleID, current); err != nil {
				return fresh, err
			}
		}
		fresh = append(fresh, added...)
	}
	return fresh, nil
}

// load returns the stored ids for ruleID and whether a record exists.
func (t *Tracker) load(ctx context.Context, ruleID string) (seen []string, known bool, err error) {
	data, err := t.kv.Get(ctx, stateKeyPrefix+ruleID)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("desktopnotify: read state for rule %q: %w", ruleID, err)
	}
	seen, known = decodeState(data)
	return seen, known, nil
}

// decodeState parses a stored record. A corrupt record is treated as absent:
// the rule then starts over silently, instead of failing on every update until
// someone deletes the record by hand.
func decodeState(data []byte) (seen []string, ok bool) {
	var st trackerState
	if json.Unmarshal(data, &st) != nil {
		return nil, false
	}
	slices.Sort(st.Seen)
	return st.Seen, true
}

func (t *Tracker) save(ctx context.Context, ruleID string, ids []string) error {
	data, err := json.Marshal(trackerState{Seen: ids})
	if err != nil {
		return fmt.Errorf("desktopnotify: encode state for rule %q: %w", ruleID, err)
	}
	if err := t.kv.Put(ctx, stateKeyPrefix+ruleID, data); err != nil {
		return fmt.Errorf("desktopnotify: write state for rule %q: %w", ruleID, err)
	}
	return nil
}
