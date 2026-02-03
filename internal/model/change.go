package model

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
