// Package flowview holds the read model of flow runs (FEAT-J0O10QI) that
// the data-entry API and the MCP tools serve: a run with why it waits, its
// timeline and what the caller may do, the caller's tasks, and the answer
// to a completed step.
//
// It is a leaf of plain values with JSON tags and imports only the
// standard library, so the transports share one shape without importing
// the composition root that fills it (RR-7RSO9I7). Whatever the values
// say has passed the caller's read gate before it lands here.
package flowview

import (
	"errors"
	"time"
)

// Flow is one flow flows.yaml declares.
type Flow struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Subject string `json:"subject"`
	Version string `json:"version"`
	// Manual reports an `on: manual` start.
	Manual bool `json:"manual"`
	// Diagram is the flow as a Mermaid state diagram.
	Diagram string `json:"diagram"`
}

// Run is one run as the caller may see it.
type Run struct {
	ID         string `json:"id"`
	Flow       string `json:"flow"`
	FlowLabel  string `json:"flow_label"`
	Version    string `json:"version"`
	Subject    string `json:"subject,omitempty"`
	State      string `json:"state"`
	StateLabel string `json:"state_label"`
	Status     string `json:"status"`
	// Generation counts the run's moves; a step names the one it saw. It
	// is 0 while the run is in parallel branches: each branch has its own.
	Generation int64     `json:"generation"`
	Started    time.Time `json:"started,omitzero"`
	Completed  time.Time `json:"completed,omitzero"`
	EnteredAt  time.Time `json:"entered_at,omitzero"`
	// Active is false for a finished run, which keeps only its header.
	Active bool `json:"active"`
	// Waiting is why an active run waits; nil for a finished one.
	Waiting *Waiting `json:"waiting,omitempty"`
	// Log is the timeline, oldest first; the start and the newest moves.
	Log []Event `json:"log,omitempty"`
	// Diagram is the flow's Mermaid diagram with the current state marked.
	Diagram string `json:"diagram,omitempty"`
	// Parallel are the branches of a run that forked (TKT-ZO9IB6V), in
	// fork order. The run's State is then the join they continue at.
	Parallel []Place `json:"parallel,omitempty"`
	Actions  Actions `json:"_actions"`
}

// Place is one parallel branch of a run: where it is and why it waits.
type Place struct {
	// Branch names the branch: the state it started in.
	Branch     string    `json:"branch"`
	State      string    `json:"state"`
	StateLabel string    `json:"state_label"`
	Status     string    `json:"status"`
	Generation int64     `json:"generation"`
	EnteredAt  time.Time `json:"entered_at,omitzero"`
	// Arrived reports a branch that waits at the join for the others.
	Arrived bool     `json:"arrived,omitempty"`
	Waiting *Waiting `json:"waiting,omitempty"`
	// Complete reports whether the caller may complete the branch's step.
	Complete bool `json:"complete"`
}

// Waiting says why a run waits and what can move it.
type Waiting struct {
	// Roles are the roles the state waits for.
	Roles []string `json:"roles,omitempty"`
	// YouHold reports whether the caller holds one of Roles on the subject.
	YouHold bool `json:"you_hold"`
	// Writes are the subject properties the step form edits.
	Writes   []string `json:"writes,omitempty"`
	Timers   []Timer  `json:"timers,omitempty"`
	Branches []Branch `json:"branches,omitempty"`
	// Pending is set while the run writes the outputs of its state.
	Pending *Pending `json:"pending,omitempty"`
	// ClaimedBy names who is completing the step right now.
	ClaimedBy string `json:"claimed_by,omitempty"`
	// Stranded is set when the run's state or definition is gone.
	Stranded bool `json:"stranded,omitempty"`
}

// Timer is one timer of the run's state.
type Timer struct {
	Name  string    `json:"name"`
	Label string    `json:"label,omitempty"`
	To    string    `json:"to"`
	Due   time.Time `json:"due,omitzero"`
}

// Branch is one change branch of the run's state.
type Branch struct {
	To string `json:"to"`
	// Fork are the states a branch that forks starts, in place of To.
	Fork  []string `json:"fork,omitempty"`
	Label string   `json:"label,omitempty"`
	Roles []string `json:"roles,omitempty"`
	Guard string   `json:"guard,omitempty"`
	// Holds is the guard's verdict on the subject now.
	Holds bool `json:"holds"`
	// Failed is set when the guard could not be evaluated.
	Failed bool `json:"failed,omitempty"`
}

// Pending is a run that has not settled in its state.
type Pending struct {
	Failures    int       `json:"failures"`
	NextAttempt time.Time `json:"next_attempt,omitzero"`
	// Job is set in a state with a job step whose attempts have begun.
	Job *Job `json:"job,omitempty"`
}

// Job is where a state's job step is.
type Job struct {
	// Phase is waiting, queued, started, finished or failed.
	Phase string `json:"phase"`
	// Failed counts the attempts that ran and failed.
	Failed int `json:"failed"`
	// Attempts is the budget: every execution, the first included.
	Attempts int `json:"attempts"`
	// FailKind is exhausted, permanent, timeout or lost once the job
	// failed for good.
	FailKind string `json:"fail_kind,omitempty"`
}

// Event is one entry of a run's timeline.
type Event struct {
	At    time.Time `json:"at"`
	State string    `json:"state"`
	// Label is the state's label, or its name when the flow lost the state.
	Label string `json:"label"`
	From  string `json:"from,omitempty"`
	Via   string `json:"via"`
	Timer string `json:"timer,omitempty"`
	By    string `json:"by,omitempty"`
	// Branch names the parallel branch that moved; empty for the run.
	Branch string `json:"branch,omitempty"`
}

// Actions are what the caller may do with a run. Each key is computed with
// the check its POST runs.
type Actions struct {
	Complete bool `json:"complete"`
	Cancel   bool `json:"cancel"`
	Retry    bool `json:"retry"`
}

// Task is a run that waits for a role the caller holds on its subject.
type Task struct {
	Run        string    `json:"run"`
	Flow       string    `json:"flow"`
	FlowLabel  string    `json:"flow_label"`
	State      string    `json:"state"`
	StateLabel string    `json:"state_label"`
	Subject    string    `json:"subject"`
	Started    time.Time `json:"started,omitzero"`
	// Branch names the parallel branch whose step it is; empty for the
	// run's own step.
	Branch string `json:"branch,omitempty"`
}

// Tasks is one page of the caller's tasks.
type Tasks struct {
	Tasks []Task `json:"tasks"`
	// Total counts the tasks found, before paging.
	Total int `json:"total"`
	// Truncated reports that more candidates exist than one call reads; a
	// flow filter narrows them.
	Truncated bool `json:"truncated"`
}

// Runs is one page of the run list.
type Runs struct {
	Runs []Run `json:"runs"`
	// Total counts the runs found, before paging.
	Total int `json:"total"`
	// Truncated reports that more runs exist than one call reads; a flow
	// or status filter narrows them.
	Truncated bool `json:"truncated"`
}

// Completed is the answer to a completed step.
type Completed struct {
	// Written are the properties the step wrote.
	Written []string `json:"written"`
	// Moved reports whether a change branch moved the run.
	Moved bool `json:"moved"`
	// State is the run's state after the step.
	State string `json:"state"`
	// Finished reports that the run ended.
	Finished bool `json:"finished"`
	// Problem is set when the properties were written but the run did not
	// follow cleanly: "guard" (a branch guard could not be evaluated, so
	// the run stays), "moved" (the run moved on meanwhile, for example by
	// a timer) or "settling" (an output step failed after the move; it is
	// retried with backoff). Empty means the step went through.
	Problem string `json:"problem,omitempty"`
}

// The values of [Completed.Problem].
const (
	ProblemGuard    = "guard"
	ProblemMoved    = "moved"
	ProblemSettling = "settling"
)

// The kinds of error a flow call answers, for a transport to map onto its
// status codes. The flow control wraps each error it returns in one.
var (
	// ErrNotFound: the run, flow or subject does not exist or is hidden.
	ErrNotFound = errors.New("not found")
	// ErrConflict: the run moved on, is settling, is claimed or finished.
	ErrConflict = errors.New("conflict")
	// ErrForbidden: the caller lacks a role or permission the flow names.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalid: the request names something the flow does not allow.
	ErrInvalid = errors.New("invalid")
)
