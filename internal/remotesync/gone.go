package remotesync

import (
	"context"
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// goner handles a linked row the source no longer lists: it detaches or
// deletes the local row, or holds it when it was edited here.
type goner struct{ *runner }

func (r goner) goneRelation(ctx context.Context, row *ledgerRow) {
	c := &r.rep.Relations
	if r.m.Remote.onGone() == OnGoneDetach {
		c.Detached++
		r.detach(row)
		return
	}
	fromKey, typ, to, ok := splitRelationAddress(row.Local)
	from, err := entity.ParseRef(fromKey)
	valid := ok && err == nil
	if valid {
		for _, end := range []string{from.String(), to} {
			readable, err := r.readable(ctx, end)
			if err != nil {
				r.gateFailed(c, row.Address, err)
				return
			}
			if !readable {
				c.Detached++
				r.issue(IssueKept, row.Address, "", "kept and detached: an endpoint is a local row the puller cannot read")
				r.detach(row)
				return
			}
		}
	}
	if row.Adopted && !r.m.Remote.Push {
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it was there before the pull")
		r.detach(row)
		return
	}
	if !valid {
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, "the ledger holds an invalid local relation key")
		return
	}
	key := entity.RelationKey{From: from.ID, FromFace: from.Face, Type: typ, To: to}
	if r.m.Remote.Push && row.Resolve != TakeRemote {
		field, edited, err := r.relationEdited(ctx, row, key)
		if err != nil {
			c.Failed++
			r.issue(IssueFailed, row.Address, row.Local, err.Error())
			return
		}
		if edited && r.keepEdited(c, row, KindRelation, typ, field) {
			return
		}
	}
	if r.o.DryRun {
		c.Deleted++
		return
	}
	if err := r.d.Writer.DeleteRelation(ctx, key); err != nil && !errors.Is(err, store.ErrNotFound) {
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	c.Deleted++
	r.l.deletes = append(r.l.deletes, r.d.Now().Unix())
	r.d.Logger.Info("remote pull deleted a relation the source no longer lists",
		"remote", r.m.Name, "run_id", r.d.RunID, "source", row.Address, "local", row.Local)
	r.l.remove(row)
}

func (r goner) goneEntity(ctx context.Context, row *ledgerRow) {
	c := &r.rep.Entities
	if r.m.Remote.onGone() == OnGoneDetach {
		c.Detached++
		r.detach(row)
		return
	}
	ours, err := r.d.Store.GetEntity(ctx, row.localRef())
	switch {
	case errors.Is(err, store.ErrNotFound):
		if !r.o.DryRun {
			r.l.remove(row)
		}
		return
	case err != nil:
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	readable, err := r.readable(ctx, ours.Ref().String())
	if err != nil {
		r.gateFailed(c, row.Address, err)
		return
	}
	if !readable {
		// Neither the row nor whether it was edited or has relations is
		// reported, and the run does not delete it.
		c.Detached++
		r.issue(IssueKept, row.Address, "", "kept and detached: the puller cannot read the local row")
		r.detach(row)
		return
	}
	if row.Adopted && !r.m.Remote.Push {
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it was there before the pull")
		r.detach(row)
		return
	}
	if field, edited := r.editedLocally(row, ours); edited && row.Resolve != TakeRemote {
		if !r.m.Remote.Push {
			c.Detached++
			r.issue(IssueKept, row.Address, row.Local, "kept and detached: owned field "+field+" was edited locally")
			r.detach(row)
			return
		}
		if r.keepEdited(c, row, "", ours.Type, field) {
			return
		}
	}
	if r.o.DryRun {
		c.Deleted++
		return
	}
	_, err = r.d.Writer.DeleteEntity(ctx, row.Local, false)
	switch {
	case errors.Is(err, entity.ErrHasRelations):
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it has local relations")
		r.detach(row)
		return
	case err != nil && !errors.Is(err, store.ErrNotFound):
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	c.Deleted++
	r.l.deletes = append(r.l.deletes, r.d.Now().Unix())
	r.d.Logger.Info("remote pull deleted an entity the source no longer lists",
		"remote", r.m.Name, "run_id", r.d.RunID, "source", row.Address, "local", row.Local)
	r.l.remove(row)
}

// editedLocally reports an owned field whose local value is not the base.
func (r goner) editedLocally(row *ledgerRow, ours *entity.Entity) (string, bool) {
	typ, _, _ := strings.Cut(row.Address, "/")
	em := r.m.Entities[typ]
	if em == nil {
		return "", true
	}
	for _, f := range em.Fields {
		b, ok := row.Base[f.baseKey()]
		if !ok || syncmerge.Digest(f.Local, oursValue(f.Local, ours)) != b {
			return f.Local.Name, true
		}
	}
	return "", false
}

// keepEdited applies the row rule of a remote with push to a row the remote
// deleted and that was edited here since the last sync. It reports whether
// the local row is kept. Under remote the row is deleted. Under local it is
// kept, detached and marked to be restored on the remote by this run's
// push. Under hold and latest it is kept, detached, and held on the
// conflict list: the remote serves no time for a delete, so latest cannot
// decide it.
func (r goner) keepEdited(c *Counts, row *ledgerRow, kind, typ, field string) bool {
	rule := r.rowRule(row)
	if rule == ConflictRemote {
		return false
	}
	r.detach(row)
	if rule == ConflictLocal && (kind != KindRelation || r.endsLinked(row)) {
		c.Detached++
		if !r.o.DryRun {
			row.Resolve = TakeLocal
		}
		r.issue(IssueKept, row.Address, row.Local,
			"the remote deleted the row, but "+field+" was edited here; restored on the remote")
		return true
	}
	c.Conflicts++
	r.c.hold(&Conflict{
		Kind: kind, Local: row.Local, Type: typ, Address: row.Address, Reason: ReasonRemoteDeleted,
		Rule: rule, Cause: CauseEdited,
	}, r.d.Now())
	r.issue(IssueConflict, row.Address, row.Local,
		"the remote deleted the row, but "+field+" was edited here; held on the conflict list")
	return true
}

// endsLinked reports whether both ends of the relation row are linked rows
// the source still lists, so the push can create the relation again.
func (r goner) endsLinked(row *ledgerRow) bool {
	from, _, to, ok := splitRelationAddress(row.Address)
	if !ok {
		return false
	}
	for _, id := range []string{from, to} {
		end := r.byID[id]
		if end == nil || end.State != stateLinked || r.goneRows[end] {
			return false
		}
	}
	return true
}

// rowRule is the row rule of row's mapped type.
func (r goner) rowRule(row *ledgerRow) string {
	if row.Kind == "R" {
		if rm := r.m.Relations[row.Type]; rm != nil {
			return rm.Rule
		}
	} else if em := r.m.Entities[row.Type]; em != nil {
		return em.Rule
	}
	return ConflictHold
}

// relationEdited reports a meta field of the local relation key whose value
// is not row's base. A relation that is gone was not edited.
func (r goner) relationEdited(
	ctx context.Context, row *ledgerRow, key entity.RelationKey,
) (field string, edited bool, err error) {
	rm := r.m.Relations[row.Type]
	if rm == nil {
		return "", true, nil
	}
	rel, err := r.d.Store.GetRelation(ctx, key)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	for _, f := range metaFields(rm.Fields) {
		b, ok := row.Base[f.baseKey()]
		if !ok || syncmerge.Digest(f.Local, oursValue(f.Local, asEntity(rel))) != b {
			return f.Local.Name, true, nil
		}
	}
	return "", false, nil
}
