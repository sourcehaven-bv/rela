package flow

// Outcome is how a [Decide] ended.
type Outcome int

const (
	// NoMove: no branch applied.
	NoMove Outcome = iota
	// Moved: a branch applied; the decision's Move is set.
	Moved
	// GuardFailed: a branch's guard failed to evaluate. Nothing moves and
	// later branches are not tried.
	GuardFailed
	// Gated: the first branch that applied was refused by the gate.
	// Nothing moves and later branches are not tried.
	Gated
)

// Rules are the checks [Decide] runs on each branch, in this order.
type Rules[B any] struct {
	// Admits is the poster rule: whether whoever caused the event may
	// fire the branch. A branch it refuses is skipped.
	// Nil: rejected; Decide calls it for every branch and panics without it.
	Admits func(B) bool
	// Holds evaluates the branch's guard against the event. A branch whose
	// guard does not hold is skipped; an error stops the decision.
	// Nil: rejected; Decide calls it for every admitted branch and panics without it.
	Holds func(B) (bool, error)
	// Gate is the last check on the chosen branch. A refusal stops the
	// decision, so a pre-check and the store follow one rule.
	// Nil: accepted; every branch passes.
	Gate func(B) bool
	// Target is the state the branch moves to.
	// Nil: rejected; Decide calls it for the branch that moves and panics without it.
	Target func(B) string
}

// Decision is what [Decide] chose. Branch is the branch that moved, that
// failed its guard, or that the gate refused; Err is the guard's error.
type Decision[B any] struct {
	Outcome Outcome
	Move    Move
	Branch  B
	Err     error
}

// Decide picks the move an event makes out of state from. branches are the
// entries the event selects out of from, in the order they are tried; the
// first one the poster rule admits and whose guard holds moves the
// instance to its target, unless the gate refuses it.
//
// A guard that fails to evaluate stops the decision: nothing moves, and
// later branches are not tried. Falling through would let an event that
// makes a guard fail, rather than not hold, reach a later branch such as a
// catch-all (fail closed, as for a single branch).
//
// The event's source is the caller's: a channel message selects the
// entries keyed by its type and binds the message in Holds. Decide reads
// only its arguments, so a store may run it under its lock.
func Decide[B any](from string, branches []B, r Rules[B]) Decision[B] {
	for _, b := range branches {
		if !r.Admits(b) {
			continue
		}
		holds, err := r.Holds(b)
		if err != nil {
			return Decision[B]{Outcome: GuardFailed, Branch: b, Err: err}
		}
		if !holds {
			continue
		}
		if r.Gate != nil && !r.Gate(b) {
			return Decision[B]{Outcome: Gated, Branch: b}
		}
		return Decision[B]{Outcome: Moved, Branch: b, Move: Move{From: from, To: r.Target(b)}}
	}
	return Decision[B]{}
}
