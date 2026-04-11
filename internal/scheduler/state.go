package scheduler

import (
	"encoding/json"
	"time"
)

// stateFile is the name of the state file within .rela/.
const stateFile = "scheduler-state.json"

// State records the last successful run time for each task.
type State struct {
	Tasks map[string]time.Time `json:"tasks"`
}

func newState() *State {
	return &State{Tasks: make(map[string]time.Time)}
}

func parseState(data []byte) *State {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupted state file — treat as empty (all tasks missed).
		return newState()
	}
	if s.Tasks == nil {
		s.Tasks = make(map[string]time.Time)
	}
	return &s
}

func (s *State) marshal() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
