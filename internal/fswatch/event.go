// Package fswatch watches files and directories for changes and reports them
// as debounced batches of [Event] values.
package fswatch

// Op represents the type of change to stored data.
type Op int

const (
	// OpCreate indicates an item was created.
	OpCreate Op = iota
	// OpModify indicates an item was modified.
	OpModify
	// OpDelete indicates an item was deleted.
	OpDelete
	// OpRename indicates an item was renamed.
	OpRename
)

// String returns a human-readable representation of the change operation.
func (op Op) String() string {
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

// Event represents a single change to stored data.
type Event struct {
	Path string
	Op   Op
}
