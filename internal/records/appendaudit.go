package records

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

// maxAuditNames bounds the owners and transactions one append line lists.
// An append holds up to [MaxBatch] rows; past this many names the line gives
// a count, and the rows themselves still name their owner.
const maxAuditNames = 20

// forbiddenOwnerError is the [ErrForbidden] of an owner the caller may read but
// not update. It keeps the owner for the denied-write line of the append and
// reads as the error it wraps.
type forbiddenOwnerError struct {
	owner string
	err   error
}

func (e forbiddenOwnerError) Error() string { return e.err.Error() }
func (e forbiddenOwnerError) Unwrap() error { return e.err }

// appended writes the one audit line of an append of recs (TKT-AAOWVET). A
// batch is one line, so a writer that batches pays one line per request, as
// an entity write does. reversed is the ledger transaction recs reverse, or
// empty for a plain append.
//
// The line is written after the append has committed, as the line of a
// change to a mutable row is, so a rolled-back append leaves none.
func (s *Service) appended(ctx context.Context, typ string, recs []Record, reversed string) {
	if len(recs) == 0 {
		return
	}
	var owners, groups []string
	seen := map[string]bool{}
	for _, r := range recs {
		if !seen["o:"+r.Owner] {
			seen["o:"+r.Owner] = true
			owners = append(owners, r.Owner)
		}
		if r.Group != "" && !seen["g:"+r.Group] {
			seen["g:"+r.Group] = true
			groups = append(groups, r.Group)
		}
	}
	var b strings.Builder
	if reversed != "" {
		fmt.Fprintf(&b, "reversed transaction %s: ", reversed)
	}
	noun := "rows"
	if len(recs) == 1 {
		noun = "row"
	}
	fmt.Fprintf(&b, "appended %d %s of %s (%s..%s); owners: %s",
		len(recs), noun, typ, recs[0].ID, recs[len(recs)-1].ID, nameList(owners))
	if len(groups) > 0 {
		fmt.Fprintf(&b, "; transactions: %s", nameList(groups))
	}
	s.write(ctx, audit.OpAppendRecords, typ, recs[0].ID, b.String())
}

// deniedAppend writes a denied-write line when the append failed with
// [ErrForbidden]: the caller may read an owner but not update it. The line
// names the owner when the error carries it, and the transaction when the
// append is a reversal. Any other error, a hidden owner's [ErrNotFound]
// among them, gets no line, so the log never shows that a hidden owner
// exists.
func (s *Service) deniedAppend(ctx context.Context, err error, typ, reversing string) {
	if !errors.Is(err, ErrForbidden) {
		return
	}
	summary := "denied: may not update the owner"
	var fo forbiddenOwnerError
	if errors.As(err, &fo) {
		summary = "denied: may not update owner " + fo.owner
	}
	if reversing != "" {
		summary += " (reversal of transaction " + reversing + ")"
	}
	s.write(ctx, audit.OpDeniedWrite, typ, "", summary+" (op="+audit.OpAppendRecords+")")
}

// nameList joins names, or gives the first [maxAuditNames] and a count of
// the rest.
func nameList(names []string) string {
	if len(names) <= maxAuditNames {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:maxAuditNames], ", "), len(names)-maxAuditNames)
}
