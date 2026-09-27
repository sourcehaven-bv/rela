package scheduler

import (
	"encoding/json"
	"time"
)

// stateFile is the legacy state document within .rela/ (or state_kv).
const stateFile = "scheduler-state.json"

// State is the legacy scheduler-state.json layout, written by releases before
// run-state moved to [schedulerstate.Store]. It is read once, imported through
// Seed, and deleted (see Scheduler.importLegacyState).
//
// Tasks holds the start time of the last successful run; Failures and
// NextRetry hold the retry ladder of a task that was failing. Older files carry
// only "tasks".
type State struct {
	Tasks     map[string]time.Time `json:"tasks"`
	Failures  map[string]int       `json:"failures,omitempty"`
	NextRetry map[string]time.Time `json:"next_retry,omitempty"`
}

func newState() *State {
	return &State{
		Tasks:     make(map[string]time.Time),
		Failures:  make(map[string]int),
		NextRetry: make(map[string]time.Time),
	}
}

// parseState reads a legacy document. A corrupt one is treated as empty: the
// cost is re-running tasks once, which is what the old parser did too.
func parseState(data []byte) *State {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return newState()
	}
	if s.Tasks == nil {
		s.Tasks = make(map[string]time.Time)
	}
	if s.Failures == nil {
		s.Failures = make(map[string]int)
	}
	if s.NextRetry == nil {
		s.NextRetry = make(map[string]time.Time)
	}
	return &s
}
