package model

// Status represents the lifecycle status of an entity
type Status string

const (
	StatusDraft      Status = "draft"
	StatusProposed   Status = "proposed"
	StatusAccepted   Status = "accepted"
	StatusDeprecated Status = "deprecated"
	StatusRejected   Status = "rejected"
	StatusRetired    Status = "retired"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusProposed, StatusAccepted, StatusDeprecated, StatusRejected, StatusRetired:
		return true
	}
	return false
}

func AllStatuses() []Status {
	return []Status{
		StatusDraft,
		StatusProposed,
		StatusAccepted,
		StatusDeprecated,
		StatusRejected,
		StatusRetired,
	}
}

// Priority represents the priority level of an entity
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

func (p Priority) IsValid() bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow:
		return true
	}
	return false
}

func AllPriorities() []Priority {
	return []Priority{
		PriorityCritical,
		PriorityHigh,
		PriorityMedium,
		PriorityLow,
	}
}
