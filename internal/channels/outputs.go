package channels

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// OutputsAction is the name of the action that carries a session notice's
// [OutputMark]. Only the store's session step creates it, and only
// [CheckOutputStep] lets it change.
const OutputsAction = "outputs"

// Output result kinds and statuses (see [OutputResult]).
const (
	OutputKindSet    = "set"
	OutputKindCreate = "create"
	OutputKindClose  = "close"
	// OutputKindTransition is the result of a mark whose transition the
	// config no longer declares, so its outputs are unknown.
	OutputKindTransition = "transition"

	OutputDone       = "done"
	OutputFailed     = "failed"
	OutputSkipped    = "skipped"
	OutputSuperseded = "superseded"
	// OutputStarted marks a create that cannot commit with its progress
	// write while it runs. A retry that finds it does not create again.
	OutputStarted = "started"
)

// MaxOutputResults caps the results of one mark: the outputs of one
// transition and the on_final close.
const MaxOutputResults = 5

// OutputMark records a session move's graph outputs (TKT-98TR9A): the
// outbox row the move writes in the same store step, and the progress of
// the writes that follow it.
//
// Writer, Roles, Unmatched, UserEntity, AnchorType, Gen, From, To, Cause,
// Job and Timer are set by the step that creates the mark and never
// change. Attempt, Claim and ClaimedAt are the claim:
// one runner at a time holds the mark, and the owning action's PendingAt
// is ClaimedAt plus the claim TTL. A claim takes the next attempt with a
// new token once PendingAt has passed; the holder then writes progress
// (same attempt and token) after each output and finally the done write.
// See [CheckOutputStep].
type OutputMark struct {
	// Writer is the cause message's author, with its original tool. On a
	// job mark it is the run_as the move wrote under, or the leader for a
	// mark that only closes.
	Writer Author `json:"writer"`
	// Roles are the writer's verified role claims at the post. A retry
	// rebuilds the writer with them. They are never on the wire.
	Roles []string `json:"roles,omitempty"`
	// Unmatched records that the writer was verified at the post but
	// matched no user entity, so a retry is judged as such a principal.
	Unmatched bool `json:"unmatched,omitempty"`
	// UserEntity records that the writer named an existing user entity at
	// the post, so a retry after the entity is removed fails.
	UserEntity bool `json:"user_entity,omitempty"`
	// AnchorType is the entity type of the channel's anchor, which decides
	// whether a set writes a face.
	AnchorType string `json:"anchor_type,omitempty"`
	// Gen is the session generation after the move.
	Gen int64 `json:"gen"`
	// From and To are the states of the move.
	From string `json:"from"`
	To   string `json:"to"`
	// Cause is the seq of the message that caused the move; zero for a
	// timer move.
	Cause int64 `json:"cause"`
	// Job marks the outputs of a job move: a timer move, or a move a
	// trigger's post fired. They write as the session's current run_as,
	// never as Writer, which then only records who the move named. Set when
	// the mark is created and never changed, so a retry cannot switch a
	// person's write to run_as or the reverse.
	Job bool `json:"job,omitempty"`
	// Timer is the timer behind a timer move; empty for a message move.
	Timer string `json:"timer,omitempty"`
	// Attempt counts the claims; zero for a mark nobody claimed yet.
	Attempt int `json:"attempt"`
	// Claim is the random token of the current claim.
	Claim string `json:"claim,omitempty"`
	// ClaimedAt is the time of the holder's last write.
	ClaimedAt time.Time `json:"claimed_at,omitzero"`
	// Close is set while the on_final close is still due.
	Close bool `json:"close,omitempty"`
	// Results holds one result per output run so far, in order.
	Results []OutputResult `json:"results,omitempty"`
}

// OutputResult is what one output, or the on_final close, did.
type OutputResult struct {
	// Kind is [OutputKindSet], [OutputKindCreate] or [OutputKindClose].
	Kind string `json:"kind"`
	// Status is [OutputDone], [OutputFailed], [OutputSkipped],
	// [OutputSuperseded] or, while a create runs, [OutputStarted].
	Status string `json:"status"`
	// Code says why an output failed: forbidden, field_denied, invalid,
	// conflict, not_found, unconfirmed, expired, transition_removed,
	// run_as_changed or error. It never quotes an error text or an id.
	Code string `json:"code,omitempty"`
	// Props are, for a set, the properties written; for an invalid
	// output, the properties the validation named.
	Props []string `json:"props,omitempty"`
	// Face is the face a set writes when the output names one.
	Face string `json:"face,omitempty"`
	// Entity is the entity a create made.
	Entity *Promoted `json:"entity,omitempty"`
	// Warnings are the warning codes of a write that succeeded.
	Warnings []string `json:"warnings,omitempty"`
}

// Clone returns a deep copy of m.
//
// Nil: accepted; returns nil.
func (m *OutputMark) Clone() *OutputMark {
	if m == nil {
		return nil
	}
	out := *m
	out.Roles = slices.Clone(m.Roles)
	out.Results = make([]OutputResult, len(m.Results))
	for i, r := range m.Results {
		r.Props = slices.Clone(r.Props)
		r.Warnings = slices.Clone(r.Warnings)
		if r.Entity != nil {
			e := *r.Entity
			r.Entity = &e
		}
		out.Results[i] = r
	}
	if len(out.Results) == 0 {
		out.Results = nil
	}
	return &out
}

// prepareOutputMark checks next's outputs mark against the action name
// and the caps, and normalizes its time. Only [OutputsAction] carries a
// mark, and it always does unless it is cleared.
func prepareOutputMark(name string, next *Action) error {
	m := next.Outputs
	switch {
	case m == nil && name == OutputsAction:
		return fmt.Errorf("%w: the %s action carries an outputs mark", ErrInvalidMessage, OutputsAction)
	case m == nil:
		return nil
	case name != OutputsAction:
		return fmt.Errorf("%w: only the %s action carries an outputs mark", ErrInvalidMessage, OutputsAction)
	}
	if err := checkAuthor(m.Writer); err != nil {
		return fmt.Errorf("outputs writer: %w", err)
	}
	if len(m.Results) > MaxOutputResults {
		return fmt.Errorf("%w: %d output results, at most %d", ErrTooLarge, len(m.Results), MaxOutputResults)
	}
	if !ValidStateName(m.From) || !ValidStateName(m.To) || m.Gen <= 0 || m.Attempt < 0 {
		return fmt.Errorf("%w: outputs mark of move %q to %q", ErrInvalidMessage, m.From, m.To)
	}
	switch {
	case (m.Cause > 0) == (m.Timer != "") || m.Cause < 0:
		return fmt.Errorf("%w: an outputs mark names either its cause or its timer", ErrInvalidMessage)
	case m.Timer != "" && !m.Job:
		return fmt.Errorf("%w: a timer's outputs mark is a job mark", ErrInvalidMessage)
	case m.Timer != "" && !ValidStateName(m.Timer):
		return fmt.Errorf("%w: outputs mark of timer %q", ErrInvalidMessage, m.Timer)
	}
	if m.AnchorType != "" {
		if err := checkPromotedType(m.AnchorType); err != nil {
			return err
		}
	}
	for _, r := range m.Results {
		if r.Entity != nil {
			if err := checkPromoted(r.Entity); err != nil {
				return err
			}
		}
	}
	raw, err := json.Marshal(m)
	if err != nil { // coverage-ignore: invariant: a mark is strings, numbers and times
		return fmt.Errorf("%w: outputs mark does not encode: %w", ErrInvalidMessage, err)
	}
	if len(raw) > MaxPropsBytes {
		return fmt.Errorf("%w: outputs mark is %d bytes, at most %d", ErrTooLarge, len(raw), MaxPropsBytes)
	}
	c := m.Clone()
	c.ClaimedAt = NormTime(c.ClaimedAt)
	next.Outputs = c
	return nil
}

// CheckOutputStep checks a SetAction against the outputs rules, after
// [CheckActionCAS] passed. Every backend runs it:
//
//   - Another action never carries a mark and never moves from pending to
//     pending ([ErrInvalidMessage]).
//   - The [OutputsAction] mark exists only on a notice: it is never taken
//     from none, never released and never left done
//     ([ErrInvalidMessage]).
//   - Writer, Roles, Unmatched, UserEntity, AnchorType, Gen, From, To and
//     Cause never change ([ErrInvalidMessage]).
//   - A claim takes attempt + 1 with a new token, once the stored
//     PendingAt has passed by next.At, and stays pending. A progress or
//     done write keeps the attempt and token. Any other step lost the claim
//     ([ErrConflict]).
//
// A done or progress write is allowed on a closed channel, because
// [CheckActionState] only refuses a step from none.
func CheckOutputStep(cur *Action, name string, expect ActionStatus, next Action) error {
	if name != OutputsAction {
		if expect == ActionPending && next.Status == ActionPending {
			return fmt.Errorf("%w: only the %s action moves from pending to pending", ErrInvalidMessage, OutputsAction)
		}
		return nil
	}
	if cur == nil || cur.Outputs == nil || expect != ActionPending {
		return fmt.Errorf("%w: the %s mark is written only with a session move", ErrInvalidMessage, OutputsAction)
	}
	if next.Status == ActionNone || next.Outputs == nil {
		return fmt.Errorf("%w: the %s mark cannot be released", ErrInvalidMessage, OutputsAction)
	}
	c, n := cur.Outputs, next.Outputs
	if c.Writer != n.Writer || !slices.Equal(c.Roles, n.Roles) || c.Unmatched != n.Unmatched ||
		c.UserEntity != n.UserEntity || c.AnchorType != n.AnchorType || c.Gen != n.Gen || c.From != n.From ||
		c.To != n.To || c.Cause != n.Cause || c.Job != n.Job || c.Timer != n.Timer {

		return fmt.Errorf("%w: the writer and move of an outputs mark never change", ErrInvalidMessage)
	}
	switch {
	case n.Attempt == c.Attempt+1:
		if n.Claim == "" || n.Claim == c.Claim {
			return fmt.Errorf("%w: a claim needs a new token", ErrConflict)
		}
		if next.At.Before(cur.PendingAt) {
			return fmt.Errorf("%w: the outputs mark is held until %s", ErrConflict, cur.PendingAt.Format(time.RFC3339))
		}
		if next.Status != ActionPending {
			return fmt.Errorf("%w: a claim keeps the mark pending", ErrInvalidMessage)
		}
	case n.Attempt == c.Attempt && n.Claim == c.Claim:
	default:
		return fmt.Errorf("%w: the outputs mark is held by another claim", ErrConflict)
	}
	return nil
}

// timerMark returns the pending outputs action the step's Mark gives for a
// move made by the step's timer, with the move's facts filled in by the
// store, or nil when the step marks nothing. It is [sessionMark] for a
// timer: the mark names the timer instead of a cause, is a job mark, and
// takes the notice time.
func timerMark(step *SessionStep, at time.Time, move SessionMove, gen int64) (*Action, error) {
	if step.Mark == nil {
		return nil, nil //nolint:nilnil // no Mark: the step marks nothing
	}
	a := step.Mark(move)
	if a == nil {
		return nil, nil //nolint:nilnil // Mark gave no action: the move marks nothing
	}
	next := *a
	next.Status, next.At = ActionPending, at
	if next.Outputs != nil {
		m := next.Outputs.Clone()
		m.Gen, m.From, m.To, m.Cause, m.Timer, m.Job = gen, move.From, move.To, 0, step.Timer, true
		next.Outputs = m
		next.By = m.Writer
	}
	prepared, err := PrepareAction(OutputsAction, next)
	if err != nil {
		return nil, err
	}
	return &prepared, nil
}

// sessionMark returns the pending outputs action the step's Mark gives for
// a move caused by cause, with the move's facts filled in by the store, or
// nil when the step marks nothing.
func sessionMark(step *SessionStep, cause Message, move SessionMove, gen int64) (*Action, error) {
	if step.Mark == nil {
		return nil, nil //nolint:nilnil // no Mark: the step marks nothing
	}
	a := step.Mark(move)
	if a == nil {
		return nil, nil //nolint:nilnil // Mark gave no action: the move marks nothing
	}
	next := *a
	next.Status, next.At = ActionPending, cause.At
	if next.Outputs != nil {
		m := next.Outputs.Clone()
		m.Gen, m.From, m.To, m.Cause = gen, move.From, move.To, cause.Seq
		next.Outputs = m
		next.By = m.Writer
	}
	prepared, err := PrepareAction(OutputsAction, next)
	if err != nil {
		return nil, err
	}
	return &prepared, nil
}
