package realtimecfg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// GuardFacts is the `session` record a transition guard reads (see
// sessionGuardEnv). Reminders is the `session.reminders` record, one
// integer counter per reminder.
type GuardFacts struct {
	State      string
	Generation int64
	EnteredAt  time.Time
	Reminders  predicate.Value
}

// GuardHolds reports whether prog, a transition's `when:`, holds for the
// message fields and the session facts at now. The decider and the load
// check of a button's preset both call it, so the two bind a guard the
// same way.
//
// Nil prog: accepted; no guard holds. An evaluation error, or a result
// that is not a boolean, does not hold and is returned as err: a guard
// fails closed. A nil operand, from an absent optional property, is no
// error: the guard just does not hold.
func GuardHolds(
	ctx context.Context, prog *predicate.Program, fields map[string]predicate.Value, facts GuardFacts, now time.Time,
) (bool, error) {
	if prog == nil {
		return true, nil
	}
	b := predicate.NewBindings()
	err := b.SetVar("message", predicate.NewRecord(fields))
	if err == nil {
		err = b.SetVar("session", predicate.NewRecord(map[string]predicate.Value{
			"state":      predicate.NewString(facts.State),
			"generation": predicate.NewInt(facts.Generation),
			"entered_at": predicate.NewDate(facts.EnteredAt),
			"reminders":  facts.Reminders,
		}))
	}
	if err == nil {
		err = predicatefns.Bind(b, now)
	}
	if err != nil {
		return false, err
	}
	v, err := prog.Eval(ctx, b)
	if errors.Is(err, predicate.ErrNilOperand) {
		// An optional property the message leaves out: a valid config
		// whose guard does not hold, not a failure to report.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	got, ok := v.(predicate.Bool)
	if !ok {
		return false, fmt.Errorf("the guard returned %T, not a boolean", v)
	}
	return got.Bool(), nil
}
