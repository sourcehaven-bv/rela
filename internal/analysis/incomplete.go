package analysis

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// IncompleteScanError reports that an analysis could not read all of
// its input: the store iterator failed part-way, so every result
// derived from that scan is a partial one.
//
// It exists because an under-count is indistinguishable from a clean
// run once the slice is returned (BUG-4KPN2M, BUG-NEQRY2). A rule that
// never saw an entity cannot have passed on it, so callers that make a
// pass/fail claim — `rela validate` above all, whose exit code gates
// merges — must treat this as a failure rather than a warning.
//
// Callers that only render a human summary may still show the partial
// results, but must say the scan was incomplete; see `rela analyze`.
type IncompleteScanError struct {
	// Op names the scan that failed, e.g. "list entities".
	Op string
	// EntityType is the queried type, empty for an all-types query.
	EntityType string
	// Err is the underlying iterator error, typically a parse failure
	// naming the offending file.
	Err error
}

func (e *IncompleteScanError) Error() string {
	var b strings.Builder
	b.WriteString("analysis: incomplete scan: ")
	b.WriteString(e.Op)
	if e.EntityType != "" {
		fmt.Fprintf(&b, " (type %q)", e.EntityType)
	}
	b.WriteString(": ")
	b.WriteString(e.Err.Error())
	return b.String()
}

func (e *IncompleteScanError) Unwrap() error { return e.Err }

// IsIncompleteScan reports whether err is, or wraps, an
// [IncompleteScanError]. Callers use it to tell "the input could not be
// fully read" apart from a genuine finding.
func IsIncompleteScan(err error) bool {
	var ise *IncompleteScanError
	return errors.As(err, &ise)
}

// IncompleteScanFiles extracts the distinct file paths named by the
// underlying read errors in err, sorted. Returns nil when no path
// could be recovered — the message is then the only detail available.
func IncompleteScanFiles(err error) []string {
	seen := map[string]bool{}
	for _, e := range flattenIncomplete(err) {
		if p := readErrorPath(e.Err); p != "" {
			seen[p] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// readErrorPath recovers the file path a store read error names, or ""
// when it names none. The fs store wraps read failures as
// "<key>: <cause>" (see fsstore.loadEntityMeta); a backend that does
// not name a file simply yields no path and the caller falls back to
// the error message.
func readErrorPath(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	i := strings.Index(msg, ": ")
	if i <= 0 {
		return ""
	}
	candidate := msg[:i]
	// A path, not a prose prefix: markdown keys are slash-separated and
	// carry an extension, and never contain spaces in this layout.
	if strings.ContainsAny(candidate, " \t") || !strings.Contains(candidate, "/") {
		return ""
	}
	return candidate
}

// flattenIncomplete collects every IncompleteScanError in err,
// including those held in a joined error.
//
// errors.As alone is not enough: it stops at the FIRST match, and a
// run that could not read three files joins three of these. The walk
// below is structural (it inspects the error tree shape) rather than a
// match against a target, which is why it type-switches on the Unwrap
// forms instead of using errors.Is/As at each step.
func flattenIncomplete(err error) []*IncompleteScanError {
	var out []*IncompleteScanError
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		//nolint:errorlint // structural tree walk, not an error match
		switch x := e.(type) {
		case *IncompleteScanError:
			out = append(out, x)
			walk(x.Err)
		case interface{ Unwrap() []error }:
			for _, sub := range x.Unwrap() {
				walk(sub)
			}
		case interface{ Unwrap() error }:
			walk(x.Unwrap())
		}
	}
	walk(err)
	return out
}
