package flow

import "time"

// The engine's own limits. They hold for every source that runs on it:
// realtime channel sessions and flows from flows.yaml alike.
const (
	// ReminderSlack is added to a reminder's next slot in the stored due
	// time, so one leader round serves several nearby slots.
	ReminderSlack = 30 * time.Second
	// MaxStates caps the states of one definition.
	MaxStates = 64
	// MaxTimers caps the timers of one state.
	MaxTimers = 8
	// MaxReminders caps the reminders of one definition.
	MaxReminders = 8
	// MinTimer is the shortest period a timer or reminder may declare.
	// Shorter ones would make the leader's rounds a busy loop.
	MinTimer = time.Minute
)
