package repository

import "github.com/Sourcehaven-BV/rela/internal/storage"

// ChangeOp represents the type of change to stored data.
type ChangeOp int

const (
	// OpCreate indicates an item was created.
	OpCreate ChangeOp = iota
	// OpModify indicates an item was modified.
	OpModify
	// OpDelete indicates an item was deleted.
	OpDelete
	// OpRename indicates an item was renamed.
	OpRename
)

// String returns a human-readable representation of the change operation.
func (op ChangeOp) String() string {
	switch op {
	case OpCreate:
		return "CREATE"
	case OpModify:
		return "MODIFY"
	case OpDelete:
		return "DELETE"
	case OpRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// ChangeEvent represents a single change to stored data.
type ChangeEvent struct {
	Path string
	Op   ChangeOp
}

// convertEvents converts storage-level change events to repository-level events.
func convertEvents(events []storage.ChangeEvent) []ChangeEvent {
	result := make([]ChangeEvent, len(events))
	for i, e := range events {
		result[i] = ChangeEvent{
			Path: e.Path,
			Op:   ChangeOp(e.Op),
		}
	}
	return result
}
