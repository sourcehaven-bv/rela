package records

import (
	"errors"
	"strings"
)

// UserText returns the text a transport shows for a failed record request,
// and whether the failure is one the caller caused. A failure that is not
// (a backend error) returns "", false: the transport logs it and shows a
// generic message, because its text may name internals.
//
// A hidden owner is [ErrNotFound], the same text as a missing one.
func UserText(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrNotFound):
		return strings.TrimPrefix(err.Error(), "records: "), true
	case errors.Is(err, ErrForbidden):
		return "you may not change the records of this entity", true
	case errors.Is(err, ErrStale):
		return "the record changed since you read it; read it again and retry", true
	case errors.Is(err, ErrGroupExists):
		return "this transaction id is already used", true
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrUnbalanced), errors.Is(err, ErrClosed):
		msg := err.Error()
		for _, s := range []error{ErrInvalid, ErrUnbalanced, ErrClosed} {
			msg = strings.ReplaceAll(msg, s.Error()+": ", "")
		}
		return msg, true
	}
	return "", false
}
