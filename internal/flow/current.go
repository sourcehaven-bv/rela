package flow

import "time"

// Current is an instance read against a definition: the state it is in,
// when it was created, and whether that state is stranded.
//
// Every engine function takes a Current from [Read], never a stored
// [Instance]: one value carries both the read state and the stored
// schedule, so a caller passes one argument where it used to pass two.
type Current struct {
	// Instance is the stored instance. An unset one reads as the initial
	// state, entered when the instance was created, with the stored
	// restart anchors and reminder records kept: a resume or a reminder
	// post stores them before the instance first moves.
	Instance
	// CreatedAt is when the instance was created: what a since: session
	// timer counts from.
	CreatedAt time.Time
	// Stranded is set when the stored state is one the definition no
	// longer declares. Nothing moves the instance until the state is
	// declared again.
	Stranded bool
}

// Read reads the stored instance inst, created at createdAt, under d.
//
// For an instance not stored yet, a decider passes the operation's clock
// time as createdAt, the time the store will record; a view passes the
// zero time, so it leaves the entered time out.
func Read(d *Definition, inst Instance, createdAt time.Time) Current {
	if inst.IsZero() {
		return Current{
			Instance: Instance{
				State: d.Initial(), EnteredAt: createdAt, Restarts: inst.Restarts, Reminders: inst.Reminders,
			},
			CreatedAt: createdAt,
		}
	}
	_, declared := d.State(inst.State)
	return Current{Instance: inst, CreatedAt: createdAt, Stranded: !declared}
}

// live returns the declared, not final state of cur. ok is false for a
// final or stranded state.
func live(d *Definition, cur Current) (State, bool) {
	st, declared := d.State(cur.State)
	if !declared || st.Final() {
		return State{}, false
	}
	return st, true
}
