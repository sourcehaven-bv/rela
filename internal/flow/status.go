package flow

// Status is what a state means to an agent that waits on the instance. The
// values are those of MCP tasks.
type Status string

// Statuses. A final state reads [StatusCompleted] by default; a state that
// is not final reads [StatusInputRequired] when it waits for roles, else
// [StatusWorking].
const (
	// StatusWorking: the instance moves on without a person named by the
	// state, through other roles, triggers or timers.
	StatusWorking Status = "working"
	// StatusInputRequired: the instance waits for a holder of one of the
	// state's waiting roles to answer.
	StatusInputRequired Status = "input_required"
	// StatusCompleted: the instance ended as intended.
	StatusCompleted Status = "completed"
	// StatusCancelled: the instance ended because someone stopped it.
	StatusCancelled Status = "cancelled"
	// StatusFailed: the instance ended in failure. A stranded instance
	// reads as failed too, so an agent stops waiting.
	StatusFailed Status = "failed"
)

// Terminal reports whether s is a status no instance leaves.
func (s Status) Terminal() bool {
	return s == StatusCompleted || s == StatusCancelled || s == StatusFailed
}
