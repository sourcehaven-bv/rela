package flow

// Fired is one trigger [Fire] evaluated: its guard held, or, when Err is
// set, its guard failed to evaluate and it did not fire.
type Fired[T any] struct {
	Trigger T
	Err     error
}

// Fire decides which of the triggers an event fires. triggers are the
// entries the event selects, in declaration order; holds evaluates one
// trigger's guard against the event.
//
// Triggers are independent, which is where Fire differs from [Decide]:
// every trigger whose guard holds fires, not only the first, and a guard
// that fails to evaluate does not fire but does not stop the others. The
// failed trigger is reported in its place with its error, so the caller can
// act on the result in declaration order.
//
// Schema automations fire through it (TKT-GS7THPA); the entity-change
// triggers of flows.yaml are meant to share it (TKT-ZRQSGCM). Fire itself
// does no I/O; holds may.
//
// Nil holds: rejected; Fire calls it for every trigger and panics without it.
func Fire[T any](triggers []T, holds func(T) (bool, error)) []Fired[T] {
	var out []Fired[T]
	for _, t := range triggers {
		ok, err := holds(t)
		switch {
		case err != nil:
			out = append(out, Fired[T]{Trigger: t, Err: err})
		case ok:
			out = append(out, Fired[T]{Trigger: t})
		}
	}
	return out
}
