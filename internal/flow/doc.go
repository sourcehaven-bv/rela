// Package flow is the session engine: a flat state machine whose instances
// move between declared states, wait on timers and post reminders.
//
// Realtime channel sessions run on it today (TKT-RRHP2N5). Later milestones
// of FEAT-J0O10QI run declared processes from flows.yaml on the same engine,
// without a channel. The package therefore knows no channel, message, ACL or
// store:
//
//   - A [Definition] is the compiled machine: its states, their timers, and
//     the reminders. The realtime.yaml loader builds one per channel type.
//   - An [Instance] is the stored state of one run of a definition. The
//     channel store keeps one per channel; its key is the store's business.
//   - [Read] reads an instance against a definition into a [Current], which
//     every engine function takes.
//   - [Decide] picks the branch an event takes out of the current state. It
//     is source-agnostic: the caller passes the branches the event selects
//     and the guard that binds the event's variables.
//   - [StepSchedule], [Resume], [NextTimer] and [NextReminder] derive the
//     due times and the stored schedule after a step.
//   - [DueWake] carries the due times writes store to the leader's timer
//     loop.
//
// Every function here is pure and fast: no I/O and no locks, except the
// mutex inside [DueWake]. The stores call the deciders under their lock.
package flow
