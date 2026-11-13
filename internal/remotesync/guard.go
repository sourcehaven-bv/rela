package remotesync

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

// guarder decides which gone rows and deletes a run may act on: the
// max_gone guard, the emptied-type check and the held deletes.
type guarder struct{ *runner }

// guard picks the linked rows the source no longer lists and the deleting
// rows the push is to delete on the remote, and drops the other gone rows
// from the ledger except the dropped ones. A deleting row the source no
// longer lists was deleted on both sides, so it is dropped from the ledger
// too. When the rows to delete or detach are too many, in this run or with
// the deletes of the last [deleteWindow], or would empty a type on either
// side, it holds them all and returns an error wrapping [ErrGoneGuard]:
// the rest of the run still applies. It writes nothing else.
func (r guarder) guard(diff reconclient.Result) (gone, sends []*ledgerRow, refused error) {
	skipped := map[string]bool{}
	for _, s := range diff.Skipped {
		if s.Kind == reconclient.KindRelation {
			skipped[s.Type] = true
		}
	}
	var forget []*ledgerRow
	r.goneRows = map[*ledgerRow]bool{}
	goneOf := map[string]int{}
	bothGone := map[*ledgerRow]bool{}
	for _, el := range diff.OnlyLocal {
		row := r.l.get(el.Kind, el.Address)
		if row == nil || (row.Kind == "R" && skipped[row.Type]) {
			continue
		}
		goneOf[row.Kind+row.Type]++
		switch row.State {
		case stateLinked:
			gone = append(gone, row)
			r.goneRows[row] = true
		case stateDropped:
			// Kept, so a local delete stays one when the source lists
			// the row again.
		case stateDeleting:
			bothGone[row] = true
			forget = append(forget, row)
		default:
			forget = append(forget, row)
		}
	}
	if !r.o.DryRun {
		for _, row := range forget {
			r.l.remove(row)
		}
	}
	for _, row := range r.l.rows {
		switch {
		case row.State == stateLinked && row.Resolve == TakeRemote && !r.goneRows[row] && !r.o.DryRun:
			// The source lists the row again: a choice to delete it here
			// has nothing left to apply to. A row still gone keeps it until
			// the gone phase uses it, so a held guard does not lose it.
			row.Resolve = ""
		case row.State == stateDeleting && !bothGone[row] && r.mapped(row) &&
			(row.Kind != "R" || !skipped[row.Type]) && !r.heldDelete(row):
			sends = append(sends, row)
		}
	}
	slices.SortFunc(sends, func(a, b *ledgerRow) int { return strings.Compare(a.Kind+a.Address, b.Kind+b.Address) })
	r.rep.Gone, r.rep.Deleting = len(gone), len(sends)
	if why := r.overGuard(diff, gone, sends, goneOf, skipped); why != "" {
		r.rep.Refused = why
		return nil, nil, fmt.Errorf("%w: %s", ErrGoneGuard, why)
	}
	return gone, sends, nil
}

// heldDelete reports whether row waits on the conflict list as a delete
// sent from here that the remote changed or refused. It is not sent again
// and does not count against the guard until a person resolves it to the
// local side.
func (r guarder) heldDelete(row *ledgerRow) bool {
	e, ok := r.c.entries[conflictKey(row.Local, "")]
	return ok && e.Reason == ReasonLocalDeleted && row.Resolve != TakeLocal
}

// mapped reports whether row's type is still in the mapping.
func (r guarder) mapped(row *ledgerRow) bool {
	if row.Kind == "R" {
		return r.m.Relations[row.Type] != nil
	}
	return r.m.Entities[row.Type] != nil
}

// overGuard says why the planned deletes and detaches are held, or "" when
// they may go ahead. --allow-gone confirms up to that many.
func (r guarder) overGuard(
	diff reconclient.Result, gone, sends []*ledgerRow, goneOf map[string]int, skipped map[string]bool,
) string {
	n := len(gone) + len(sends)
	if n == 0 || n <= r.o.AllowGone {
		return ""
	}
	limit := r.m.Remote.maxGone()
	what := fmt.Sprintf("%d linked rows are gone from the source and %d rows deleted here wait to be deleted on "+
		"the remote", len(gone), len(sends))
	if n > limit {
		return fmt.Sprintf("%s, over max_gone %d; confirm with --allow-gone %d", what, limit, n)
	}
	deletes := len(sends)
	if r.m.Remote.onGone() == OnGoneDelete {
		deletes += len(gone)
	}
	if recent := r.l.recentDeletes(r.d.Now()); deletes > 0 && recent+deletes > limit {
		return fmt.Sprintf("%s, and %d rows were deleted in the last %s, over max_gone %d; "+
			"confirm with --allow-gone %d", what, recent, deleteWindow, limit, n)
	}
	if t, ok := r.emptied(diff, goneOf, skipped); ok {
		return fmt.Sprintf("the source lists no rows of %s any more; confirm with --allow-gone %d", t, n)
	}
	if t, ok := r.emptiedHere(sends); ok {
		return fmt.Sprintf("every linked row of %s was deleted here; confirm with --allow-gone %d", t, n)
	}
	return ""
}

// emptiedHere returns a type whose every linked row was deleted here, so
// sending the deletes would empty it on the remote. It is the local side's
// form of [runner.emptied].
func (r guarder) emptiedHere(sends []*ledgerRow) (string, bool) {
	deleting, linked := map[string]int{}, map[string]int{}
	for _, row := range sends {
		deleting[row.Kind+row.Type]++
	}
	for _, row := range r.l.rows {
		if row.State == stateLinked {
			linked[row.Kind+row.Type]++
		}
	}
	for _, k := range sortedKeys(deleting) {
		if linked[k] == 0 {
			kind := "entity type"
			if k[0] == 'R' {
				kind = "relation type"
			}
			return fmt.Sprintf("%s %q", kind, k[1:]), true
		}
	}
	return "", false
}

// emptied returns a type that had linked rows and that the source now lists
// no rows of. A type the source stopped serving, through its ACL or a
// broken mapping, looks like that; deleting its rows on that evidence is the
// mass delete the guard exists for.
func (r guarder) emptied(diff reconclient.Result, goneOf map[string]int, skipped map[string]bool) (string, bool) {
	listed, linked := map[string]int{}, map[string]int{}
	for _, rows := range r.l.index {
		for _, row := range rows {
			k := row.Kind + row.Type
			listed[k]++
			if row.State == stateLinked {
				linked[k]++
			}
		}
	}
	for _, el := range diff.OnlyRemote {
		listed[kindLetter(el.Kind)+elementType(el)]++
	}
	for _, k := range sortedKeys(linked) {
		if linked[k] > 0 && listed[k]-goneOf[k] <= 0 && (k[0] != 'R' || !skipped[k[1:]]) {
			kind := "entity type"
			if k[0] == 'R' {
				kind = "relation type"
			}
			return fmt.Sprintf("%s %q", kind, k[1:]), true
		}
	}
	return "", false
}
