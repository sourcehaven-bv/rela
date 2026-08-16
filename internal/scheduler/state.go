package scheduler

import (
	"encoding/json"
	"time"
)

// stateFile is the name of the state file within .rela/.
const stateFile = "scheduler-state.json"

// State records per-task scheduling state.
//
// Tasks holds the start time of the last *successful* run — it is what the
// schedule is evaluated against, so a failed attempt must never advance it.
// Failures and NextRetry hold the retry ladder for a task that is currently
// failing: while NextRetry is set, it is the ONLY thing that triggers the
// task, and the ordinary schedule is suppressed (see runDueTasks).
//
// Splitting "when did this last succeed" from "is this currently failing" is
// deliberate. Encoding both in Tasks alone is what caused BUG-ZKK2UL: a
// failure left no trace at all, so the task stayed perpetually due and retried
// at the tick rate.
//
// The three maps are separate JSON fields rather than one map of structs so
// that a state file written by an older binary — which has only "tasks" —
// still parses. That is the only compatibility property claimed here.
//
// It is NOT forward compatible: saveState marshals the whole struct, so an
// older binary that writes this file drops the fields it does not know, and a
// downgrade (or mixed-version rollout) resets every in-flight ladder. A
// map-of-structs layout would behave identically, so the split buys backward
// compatibility only. The consequence is bounded — a reset ladder retries from
// the first rung — and the file lives in gitignored .rela/.
//
// The retry maps carry omitempty, so a healthy state file is byte-identical to
// a pre-retry one: absent and empty mean the same thing, and parseState
// nil-guards each map independently.
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

func parseState(data []byte) *State {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupted state file — treat as empty (all tasks missed).
		return newState()
	}
	// Each map is nil-guarded independently: a file written before retry
	// state existed carries only "tasks", and the omitempty fields are
	// absent from any file where no task is currently failing.
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

func (s *State) marshal() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
