package remotesync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// On a remote with send_deletes, a linked row deleted here becomes a
// deleting row (see [runner.sweep]). After the entity and relation push,
// the run deletes each such row on the remote, relations first, so an
// entity's relations are gone before it is:
//
//   - the remote row is fetched with its ETag. A 404 means it is gone
//     already, and the row is forgotten.
//   - a mapped field that moved off the merge base is a remote edit since
//     the last sync. The row rule decides it: under hold and latest the row
//     is held on the conflict list as [ReasonLocalDeleted], under remote it
//     is forgotten so the next pull restores it here, under local the
//     delete is sent anyway. A row a person resolved to local skips the
//     check.
//   - otherwise the row is deleted by DELETE with If-Match on the fetched
//     ETag. A 2xx or 404 forgets the row; a 412 fetches it again, once. A
//     400, 403, 409 or 422 holds it on the conflict list; anything else
//     fails it, and the next run tries again.
//
// The remote applies its own rules to the delete, as for any caller of its
// API: its ACL, and its cascade or soft delete.

// deleter runs the delete phase of a remote with push. It shares the run's
// state.
type deleter struct{ *runner }

// send deletes the rows in sends on the remote. The gone guard has passed
// them; sends is empty when it held them.
func (r deleter) send(ctx context.Context, sends []*ledgerRow) error {
	for _, kind := range []string{"R", "E"} {
		for _, row := range sends {
			if row.Kind != kind {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			var err error
			if kind == "R" {
				err = r.sendRelation(ctx, row)
			} else {
				err = r.sendEntity(ctx, row)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// remoteRow is the remote side of one row a delete is about: its route,
// the fields to compare with the base, and how to fetch it.
type remoteRow struct {
	pc     *PushCounts
	kind   string
	typ    string
	path   string
	fields []FieldMapping
	rule   string
	// fetch reads the row's owned fields and ETag. A gone row wraps
	// [httpremote.ErrNotFound].
	fetch func(ctx context.Context) ([]theirsValue, string, error)
}

func (r deleter) sendEntity(ctx context.Context, row *ledgerRow) error {
	em := r.m.Entities[row.Type]
	id, ok := addressID(row.Address)
	if em == nil || !ok { // coverage-ignore: invariant: guard sends only mapped rows with an address
		return nil
	}
	plural := r.plurals[em.Source]
	return r.sendOne(ctx, row, remoteRow{
		pc: r.rep.Pushed, typ: em.Local, path: entityPath(plural, id), fields: em.Fields, rule: em.Rule,
		fetch: func(ctx context.Context) ([]theirsValue, string, error) {
			re, etag, err := fetchTagged(ctx, r, plural, em.Source, id)
			if err != nil {
				return nil, "", err
			}
			th, err := theirs(em.Fields, re.Properties, re.Content, re.Redacted)
			return th, etag, err
		},
	})
}

func (r deleter) sendRelation(ctx context.Context, row *ledgerRow) error {
	rm := r.m.Relations[row.Type]
	from, typ, to, ok := splitRelationAddress(row.Address)
	fromRow := r.byID[from]
	if rm == nil || !ok || fromRow == nil || r.plurals[fromRow.Type] == "" {
		r.rep.PushedRelations.Failed++
		r.issue(IssueFailed, row.Address, row.Local, "delete: the remote route of the relation is not known")
		return nil
	}
	path := r.plurals[fromRow.Type] + "/" + url.PathEscape(from) + "/relations/" + url.PathEscape(typ) +
		"/" + url.PathEscape(to)
	return r.sendOne(ctx, row, remoteRow{
		pc: r.rep.PushedRelations, kind: KindRelation, typ: rm.Local, path: path,
		fields: metaFields(rm.Fields), rule: rm.Rule,
		fetch: func(ctx context.Context) ([]theirsValue, string, error) {
			re, etag, err := fetchRelationTagged(ctx, r, path, from, typ, to)
			if err != nil {
				return nil, "", err
			}
			th, err := theirs(metaFields(rm.Fields), re.Meta, re.Content, re.Redacted)
			return th, etag, err
		},
	})
}

// sendOne deletes one row on the remote.
func (r deleter) sendOne(ctx context.Context, row *ledgerRow, rr remoteRow) error {
	if r.o.DryRun {
		rr.pc.Deleted++
		return nil
	}
	for range maxAttempts {
		if !r.deleteBudget(rr.pc) {
			return nil
		}
		th, etag, err := rr.fetch(ctx)
		switch {
		case errors.Is(err, httpremote.ErrNotFound):
			r.forget(row)
			return nil
		case errors.Is(err, errProtocol):
			rr.pc.Failed++
			r.issue(IssueRejected, row.Address, row.Local, "delete: "+err.Error())
			return nil
		case err != nil:
			return r.deleteFailed(rr.pc, row, err)
		case etag == "":
			rr.pc.Failed++
			r.issue(IssueFailed, row.Address, row.Local,
				"delete: the remote sent no ETag for the row, so it cannot be deleted safely")
			return nil
		}
		if field, edited := remoteEdited(rr.fields, row, th); edited && row.Resolve != TakeLocal &&
			r.keepRemote(rr, row, field) {

			return nil
		}
		if !r.deleteBudget(rr.pc) {
			return nil
		}
		ans, err := r.Send(ctx, http.MethodDelete, rr.path, nil, etag, maxPushAnswer)
		if err != nil {
			return r.deleteFailed(rr.pc, row, err)
		}
		switch {
		case ans.Status == http.StatusNotFound:
			// Gone already, as on a fetch that answers 404.
			r.forget(row)
			return nil
		case ans.Status >= 200 && ans.Status < 300:
			rr.pc.Deleted++
			r.l.deletes = append(r.l.deletes, r.d.Now().Unix())
			r.d.Logger.Info("remote push deleted a row deleted here",
				"remote", r.m.Name, "run_id", r.d.RunID, "remote_row", row.Address, "local", row.Local)
			r.forget(row)
			return nil
		case ans.Status == http.StatusPreconditionFailed:
			continue
		}
		r.deleteRefusedOrFailed(ans, rr, row)
		return nil
	}
	rr.pc.Conflicts++
	r.issue(IssueConflict, row.Address, row.Local, "the remote row changed again during the delete; tried again next run")
	return nil
}

// remoteEdited reports a field of the fetched remote row th whose value is
// not row's base, the value both sides last agreed on.
func remoteEdited(fields []FieldMapping, row *ledgerRow, th []theirsValue) (string, bool) {
	for _, t := range th {
		b, ok := row.Base[t.f.baseKey()]
		if !ok || syncmerge.Digest(t.f.Local, t.v) != b {
			return t.f.Local.Name, true
		}
	}
	for _, f := range fields {
		if _, ok := row.Base[f.baseKey()]; !ok {
			return f.Local.Name, true
		}
	}
	return "", false
}

// keepRemote applies the row rule to a row deleted here that the remote
// changed since the last sync. It reports whether the remote row is kept.
func (r deleter) keepRemote(rr remoteRow, row *ledgerRow, field string) bool {
	switch rr.rule {
	case ConflictLocal:
		return false
	case ConflictRemote:
		r.forget(row)
		r.issue(IssueKept, row.Address, row.Local,
			"deleted here, but the remote changed "+field+"; restored here on the next pull")
		return true
	}
	rr.pc.Conflicts++
	r.c.hold(&Conflict{
		Kind: rr.kind, Local: row.Local, Type: rr.typ, Address: row.Address, Reason: ReasonLocalDeleted,
		Rule: rr.rule, Cause: CauseEdited,
	}, r.d.Now())
	r.issue(IssueConflict, row.Address, row.Local,
		"deleted here, but the remote changed "+field+"; held on the conflict list")
	return true
}

// deleteRefusedOrFailed handles a delete answer that is neither a success,
// a 404 nor a 412.
func (r deleter) deleteRefusedOrFailed(ans httpremote.Answer, rr remoteRow, row *ledgerRow) {
	detail := reconclient.Sanitize(string(ans.Body))
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	switch ans.Status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity:
		rr.pc.Refused++
		r.c.hold(&Conflict{
			Kind: rr.kind, Local: row.Local, Type: rr.typ, Address: row.Address, Reason: ReasonLocalDeleted,
			Detail: fmt.Sprintf("%d %s", ans.Status, detail), Cause: CauseRefused, Status: ans.Status,
		}, r.d.Now())
		r.issue(IssueRefused, row.Address, row.Local,
			fmt.Sprintf("the remote refused the delete (%d): %s; held on the conflict list", ans.Status, detail))
	default:
		rr.pc.Failed++
		r.issue(IssueFailed, row.Address, row.Local, fmt.Sprintf("delete: the remote answered %d: %s", ans.Status, detail))
	}
}

// forget drops a row whose remote row is gone or is to be restored here.
// The next pull of a remote row that is still there restores it.
func (r deleter) forget(row *ledgerRow) {
	r.l.remove(row)
	delete(r.l.held, rowKey(row.kind(), row.Address))
}

// deleteBudget reports whether the run may send one more delete request,
// counted against max_fetch and max_bytes like a fetch.
func (r deleter) deleteBudget(pc *PushCounts) bool {
	if r.fetched >= r.m.Remote.maxFetch() || r.read >= r.m.Remote.maxBytes() {
		pc.Deferred++
		r.rep.Waiting++
		return false
	}
	r.fetched++
	return true
}

// deleteFailed classifies an error of a delete request as
// [pusher.pushFailed] does a push's. A delete whose outcome is unknown is
// fetched again by the next run, which finds the row gone or not.
func (r deleter) deleteFailed(pc *PushCounts, row *ledgerRow, err error) error {
	var refused *reconclient.RefusedError
	switch {
	case errors.Is(err, httpremote.ErrDenied):
		return fmt.Errorf("remotesync: the remote refused the push's credential: %w", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.As(err, &refused):
		return fmt.Errorf("remotesync: the remote is busy; push stopped: %w", err)
	}
	pc.Failed++
	r.issue(IssueFailed, row.Address, row.Local, "delete: "+err.Error())
	return nil
}
