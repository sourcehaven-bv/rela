package dataentry

import (
	"errors"
	"fmt"
)

// relationError surfaces a per-edge failure with the relation type and
// target id attached so the handler layer can produce a problem-details
// response the UI can attribute to the specific chip that caused it.
// Returned by [writeHandler.applyRelationsModern] (the only relation reconciler
// after the legacy IDs-only shape was dropped).
type relationError struct {
	RelType string
	Target  string
	Op      string // "create" | "delete" | "validate"
	Reason  string // stable reason code
	Err     error  // underlying error, for Unwrap / logs
}

func (e *relationError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("relation %s %s %s: %s", e.Op, e.RelType, e.Target, e.Reason)
	}
	return fmt.Sprintf("relation %s %s: %s", e.Op, e.RelType, e.Reason)
}

func (e *relationError) Unwrap() error { return e.Err }

// reconcileDetail formats a reconcile error for the problem-details
// response as "relation=<t> op=<op> reason=<reason> target=<to>". Every
// field came from the request. The underlying cause is left out, because a
// store error can name data the caller may not read (GitHub #1774); it is
// empty when err is not a *relationError.
func reconcileDetail(err error) string {
	var rerr *relationError
	if !errors.As(err, &rerr) {
		return ""
	}
	base := fmt.Sprintf("relation=%s op=%s reason=%s", rerr.RelType, rerr.Op, rerr.Reason)
	if rerr.Target != "" {
		base += " target=" + rerr.Target
	}
	return base
}
