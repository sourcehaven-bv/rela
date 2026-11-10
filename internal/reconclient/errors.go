package reconclient

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// RefusedError is an admission refusal by the remote (429 or 503): the
// request was not run and may be sent again after RetryAfter.
type RefusedError struct {
	Status int
	// RetryAfter is the wait the remote asked for; zero when it named none.
	RetryAfter time.Duration
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("reconcile: remote refused the request (status %d, retry after %s)", e.Status, e.RetryAfter)
}

// RangeError is a refusal of one range of a request. Nothing of the request
// was answered.
type RangeError struct {
	// Index is the range's index in the request, or -1 when the remote did
	// not say.
	Index int
	// MetaGated reports that the remote's policy can hide a meta key of a
	// relation type of the range, so it serves no elements of that type.
	MetaGated bool
	Status    int
	// Detail is the remote's explanation, already passed through
	// [Sanitize].
	Detail string
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("reconcile: remote refused range %d (status %d): %s", e.Index, e.Status, e.Detail)
}

// Errors the Differ returns. Each is wrapped with detail.
var (
	// ErrBudget reports that the run hit one of its [Options] limits.
	ErrBudget = errors.New("reconcile: budget exceeded")
	// ErrProtocol reports a remote answer that breaks the contract.
	ErrProtocol = errors.New("reconcile: invalid answer from remote")
)

// maxDetail caps the runes of a remote's text kept by [Sanitize].
const maxDetail = 300

// Sanitize returns s with every rune that is not printable, or is a format
// character such as a bidi override, replaced by U+FFFD, and cut to 300
// runes. Text a remote sends passes through it before it reaches an error or
// a terminal.
func Sanitize(s string) string {
	var b strings.Builder
	n := 0
	for s != "" && n < maxDetail {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if !printable(r) {
			r = utf8.RuneError
		}
		b.WriteRune(r)
		n++
	}
	if s != "" {
		b.WriteString("...")
	}
	return b.String()
}

// printable reports whether r may be shown as is: a printable rune that is
// not a format character (Cf holds the bidi overrides and zero-width
// characters).
func printable(r rune) bool {
	return r != utf8.RuneError && unicode.IsPrint(r) && !unicode.Is(unicode.Cf, r)
}

// Printable reports whether every rune of s may be shown as is. Text output
// quotes a value for which it is false.
func Printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !printable(r) {
			return false
		}
	}
	return true
}
