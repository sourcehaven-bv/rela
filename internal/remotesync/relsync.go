package remotesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// On a remote with push, relation meta flows both ways (Q5 of TKT-8ZGAEC5).
// The pull merges the owned meta of a linked relation three ways, under the
// same conflict rules as entity fields; the body still comes from the
// source. After the entity push, the relation push sends each local relation
// of a mapped type whose two endpoints are linked and readable:
//
//   - a relation the ledger does not know is created on the remote by POST
//     on its source endpoint. A 409 means the remote has it already; the
//     next pull links the two and merges their meta.
//   - a linked relation whose owned meta moved off the merge base is fetched
//     with its ETag, merged as a pull does, and sent by PATCH with If-Match.
//     A cleared value is sent as null; the remote keeps the key with a null value.
//     A 412 tries once more.
//
// A 400, 403, 409 on an update, or 422 holds the relation on the conflict
// list. A type whose meta has `visible:` grants is neither merged nor pushed:
// the run cannot tell which of its meta the principal may read. A relation
// is never deleted on the remote.

// relSync runs the relation half of a remote with push. It shares the run's
// state.
type relSync struct{ *runner }

// merges reports whether the run merges and pushes rm's meta: the remote
// has push and the local type's meta has no `visible:` grants.
func (r relSync) merges(rm *RelationMapping) bool {
	return r.m.Remote.Push && r.d.RelationMetaGated != nil && !r.d.RelationMetaGated(rm.Local)
}

// metaFields returns the fields of a relation mapping that flow both ways:
// all but the body.
func metaFields(fields []FieldMapping) []FieldMapping {
	out := make([]FieldMapping, 0, len(fields))
	for _, f := range fields {
		if f.Local.Kind != syncmerge.KindContent {
			out = append(out, f)
		}
	}
	return out
}

// metaBase returns the merge base a relation write leaves: the digest of
// each reported meta field.
func metaBase(th []theirsValue) map[string]string {
	out := map[string]string{}
	for _, t := range th {
		if t.f.Local.Kind != syncmerge.KindContent {
			out[t.f.baseKey()] = syncmerge.Digest(t.f.Local, t.v)
		}
	}
	return out
}

// asEntity presents a relation's meta and body as an entity, the shape the
// merge reads.
func asEntity(rel *entity.Relation) *entity.Entity {
	return &entity.Entity{Properties: rel.Properties, Content: rel.Content}
}

// mergeRelation merges a fetched source relation into the existing local
// one: the owned meta three ways, the body source-wins. It reports whether
// the merge was recorded: false when the local write was rejected, failed,
// or lost a race, or on a dry run. The push must not send the relation
// then, because the local side does not hold what the merge decided.
func (r relSync) mergeRelation(
	ctx context.Context, rm *RelationMapping, row *ledgerRow, addr string, hash canonical.Digest,
	key entity.RelationKey, existing *entity.Relation, src sourceRow,
) bool {
	c := &r.rep.Relations
	var meta, body []theirsValue
	for _, t := range src.th {
		if t.f.Local.Kind == syncmerge.KindContent {
			body = append(body, t)
		} else {
			meta = append(meta, t)
		}
	}
	var synced time.Time
	if row != nil {
		synced = row.Synced
	}
	v := latest(r.relStamp(key, existing), src.updated, synced, r.d.Now(), r.m.Remote.maxClockSkew())
	mg := merge(metaFields(rm.Fields), baseOf(row), asEntity(existing), meta, v)
	local := key.String()
	r.noteConflicts(conflictTarget{typ: rm.Local, local: local, addr: addr, kind: KindRelation}, mg)
	c.Settled += len(mg.settled)
	if len(mg.conflicts) > 0 {
		c.Conflicts++
		r.issue(IssueConflict, addr, local, "both sides changed these fields; held on the conflict list",
			mg.conflictNames()...)
	}
	opts := relationChanges(existing, body)
	opts.Properties, opts.MetaUnset = mg.props, mg.unset
	changed := len(opts.Properties) > 0 || len(opts.MetaUnset) > 0 || opts.Content != nil
	linked := row != nil && row.State == stateLinked && row.Local == local
	adopted := row == nil || row.Local != local || row.Adopted
	if changed {
		if err := r.checkRelationMeta(key.Type, existing, opts); err != nil {
			c.Rejected++
			r.issue(IssueRejected, addr, local, err.Error())
			r.record(row, "R", rm.Source, addr, hash, local, stateRejected, nil)
			return false
		}
	}
	switch {
	case !changed && len(mg.conflicts) > 0:
	case !changed && linked:
		c.Unchanged++
	case !changed:
		c.Adopted++
	case r.o.DryRun:
		c.Updated++
		return false
	default:
		opts.ExpectedVersion = string(store.RelationVersionOf(existing))
		_, err := r.d.Writer.UpdateRelation(ctx, key, opts)
		if _, conflict := errors.AsType[*store.VersionConflictError](err); conflict {
			// The local relation changed under the write: the next run
			// compares it again.
			c.Deferred++
			r.hold()
			return false
		}
		if err != nil {
			r.writeFailed(c, addr, local, err)
			if rejectedWrite(err) {
				r.record(row, "R", rm.Source, addr, hash, local, stateRejected, nil)
			}
			return false
		}
		if linked {
			c.Updated++
		} else {
			c.Adopted++
		}
	}
	rec := r.record(row, "R", rm.Source, addr, hash, local, stateLinked, mg.base)
	rec.adopt("", adopted)
	r.markSynced(rec, local)
	return true
}

// fetchSource fetches the source relation at addr when the mapping maps
// any of its fields, and returns its mapped values and last-updated time.
// A relation with no mapped field is not fetched. ok is false when the
// pull of this relation stops here: the fetch budget ran out, the fetch
// failed, or the answer was rejected; err is then the error that stops the
// run, if any.
func (r relSync) fetchSource(
	ctx context.Context, row *ledgerRow, rm *RelationMapping, addr string, hash canonical.Digest,
	key entity.RelationKey, plural string,
) (src sourceRow, ok bool, err error) {
	if len(rm.Fields) == 0 {
		return sourceRow{}, true, nil
	}
	c := &r.rep.Relations
	if !r.budget(c) {
		return sourceRow{}, false, nil
	}
	from, typ, to, _ := splitRelationAddress(addr)
	rel, err := fetchRelation(ctx, r, plural, from, typ, to)
	if err != nil {
		rejected, runErr := r.fetchFailed(c, "relation", addr, err)
		if rejected {
			r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
		}
		return sourceRow{}, false, runErr
	}
	th, err := theirs(rm.Fields, rel.Meta, rel.Content, rel.Redacted)
	if err != nil {
		c.Rejected++
		r.issue(IssueRejected, addr, key.String(), err.Error())
		r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
		return sourceRow{}, false, nil
	}
	return sourceRow{th: th, updated: parseStamp(rel.Updated)}, true, nil
}

// relStamp returns the local relation's last-updated time as the latest
// rule may use it: the time it had when the run first saw it. The run reads
// relations whose two endpoints the principal may read, of a type without
// `visible:` meta, so nothing on it is hidden.
func (r relSync) relStamp(key entity.RelationKey, rel *entity.Relation) stamp {
	k := "R\x00" + key.String()
	at, seen := r.localTimes[k]
	if !seen {
		at = rel.UpdatedAt
		if r.localTimes == nil {
			r.localTimes = map[string]time.Time{}
		}
		r.localTimes[k] = at
	}
	return stamp{At: at}
}

// push sends the local relations of every mapped type.
func (r relSync) push(ctx context.Context) error {
	run := relPushRun{ends: map[string]*ledgerRow{}, seen: map[string]bool{}, readable: map[string]bool{}}
	for _, row := range r.l.rows {
		if row.Kind == "E" && row.State == stateLinked && row.Local != "" {
			run.ends[row.Local] = row
		}
	}
	for _, src := range sortedKeys(r.m.Relations) {
		rm := r.m.Relations[src]
		if !r.merges(rm) {
			r.issue(IssueKept, "", "", notPushedReason(r.d.RelationMetaGated == nil, rm.Local))
			continue
		}
		var rels []*entity.Relation
		for rel, err := range r.d.Store.ListRelations(ctx, store.RelationQuery{Type: rm.Local}) {
			if err != nil {
				return fmt.Errorf("remotesync: list local %s relations: %w", rm.Local, err)
			}
			rels = append(rels, rel)
		}
		for _, rel := range rels {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := r.pushRelation(ctx, rm, rel, run); err != nil {
				return err
			}
		}
	}
	r.relSeen = run.seen
	return nil
}

// relPushRun is the state one relation push shares across relations: the
// linked entity rows by local id, the relation keys already handled, and
// whether each endpoint is readable, so an endpoint is checked once per run.
type relPushRun struct {
	ends     map[string]*ledgerRow
	seen     map[string]bool
	readable map[string]bool
}

// notPushedReason says why the relations of a type are not pushed.
func notPushedReason(noCheck bool, relType string) string {
	if noCheck {
		return fmt.Sprintf("relations of type %s are not pushed: this run cannot check its meta for visible: grants",
			relType)
	}
	return fmt.Sprintf("relations of type %s are not pushed: its meta has visible: grants, "+
		"so the run cannot tell which of it the pusher may read", relType)
}

// relTarget is a local relation the push may send, with its remote address
// and the remote ids of its endpoints.
type relTarget struct {
	rm           *RelationMapping
	key          entity.RelationKey
	addr         string
	fromRow      *ledgerRow
	fromID, toID string
	fields       []FieldMapping
}

// pushRelation pushes one local relation when both endpoints are linked
// and readable. Any other relation is left out without naming it.
func (r relSync) pushRelation(
	ctx context.Context, rm *RelationMapping, rel *entity.Relation, run relPushRun,
) error {
	key := rel.Identity()
	from, to := run.ends[key.From], run.ends[key.To]
	if key.FromFace != "" || from == nil || to == nil || from.Face != "" {
		return nil
	}
	run.seen[key.String()] = true
	for _, end := range []*ledgerRow{from, to} {
		ref := end.localRef().String()
		readable, known := run.readable[ref]
		if !known {
			var err error
			if readable, err = r.readable(ctx, ref); err != nil {
				r.gateFailed(&r.rep.Relations, "", err)
				return nil
			}
			run.readable[ref] = readable
		}
		if !readable {
			return nil
		}
	}
	fromID, _ := addressID(from.Address)
	toID, _ := addressID(to.Address)
	t := relTarget{
		rm: rm, key: key, addr: fromID + "--" + rm.Source + "--" + toID, fromRow: from,
		fromID: fromID, toID: toID, fields: metaFields(rm.Fields),
	}
	row := r.l.get(reconclient.KindRelation, t.addr)
	switch {
	case row == nil:
		return r.pushCreateRelation(ctx, t, rel)
	case row.State == stateLinked && row.Local == key.String():
		return r.pushUpdateRelation(ctx, t, rel, row)
	}
	return nil
}

// relPath is the route of the relations of one type on a remote entity, or
// of one of them when to is set.
func (r relSync) relPath(t relTarget, to string) string {
	p := r.plurals[t.fromRow.Type] + "/" + url.PathEscape(t.fromID) + "/relations/" + url.PathEscape(t.rm.Source)
	if to != "" {
		p += "/" + url.PathEscape(to)
	}
	return p
}

// relBody is the meta of a relation create or update under the remote
// names, with the digests the push leaves as the merge base. A cleared
// field is null on an update and left out of a create.
func relBody(
	fields []FieldMapping, rel *entity.Relation, create bool,
) (meta map[string]any, digests map[string]string, err error) {
	meta, digests = map[string]any{}, map[string]string{}
	e := asEntity(rel)
	for _, f := range fields {
		v, cErr := syncmerge.Canonical(f.Local, oursValue(f.Local, e))
		if cErr != nil {
			return nil, nil, cErr
		}
		digests[f.baseKey()] = syncmerge.Digest(f.Local, v)
		if v != nil || !create {
			meta[f.Remote] = v
		}
	}
	return meta, digests, nil
}

// pushCreateRelation creates a local relation on the remote.
func (r relSync) pushCreateRelation(ctx context.Context, t relTarget, rel *entity.Relation) error {
	pc := r.rep.PushedRelations
	local := t.key.String()
	if r.c.rowHeld(t.fields, local, asEntity(rel)) {
		return nil
	}
	if r.o.DryRun {
		pc.Created++
		return nil
	}
	meta, digests, err := relBody(t.fields, rel, true)
	if err != nil {
		return r.relFailed(t.addr, local, "push: "+err.Error())
	}
	doc := map[string]any{"id": t.toID}
	if len(meta) > 0 {
		doc["meta"] = meta
	}
	body, err := json.Marshal(doc)
	if err != nil { // coverage-ignore: canonical values always marshal
		return r.relFailed(t.addr, local, "push: "+err.Error())
	}
	if !r.relBudget() {
		return nil
	}
	ans, err := r.Send(ctx, http.MethodPost, r.relPath(t, ""), body, "", maxPushAnswer)
	if err != nil {
		// A relation is one per triple on the remote, so a create that may
		// have landed is not held: the next pull links it, or the next push
		// sends it again.
		return r.relSendFailed(t.addr, local, err)
	}
	switch {
	case ans.Status >= 200 && ans.Status < 300:
		r.l.put(&ledgerRow{
			Kind: "R", Type: t.rm.Source, Address: t.addr, Local: local, State: stateLinked,
			Base: digests, Adopted: true, Synced: r.d.Now(),
		})
		pc.Created++
	case ans.Status == http.StatusConflict:
		// The remote has the relation already; the next pull links it.
		pc.Deferred++
	default:
		r.relRefusedOrFailed(ans, t, local, digests)
	}
	return nil
}

// pushUpdateRelation sends the owned meta of a linked relation that moved
// off the merge base.
func (r relSync) pushUpdateRelation(ctx context.Context, t relTarget, rel *entity.Relation, row *ledgerRow) error {
	pc := r.rep.PushedRelations
	local := t.key.String()
	if r.c.rowHeld(t.fields, local, asEntity(rel)) || len(r.c.changed(t.fields, local, asEntity(rel), row)) == 0 {
		return nil
	}
	if r.o.DryRun {
		pc.Updated++
		return nil
	}
	for range maxAttempts {
		var etag string
		var ok bool
		var err error
		if rel, etag, ok, err = r.refreshRelation(ctx, t, row); err != nil || !ok {
			return err
		}
		fields := r.c.changed(t.fields, local, asEntity(rel), row)
		if len(fields) == 0 {
			return nil
		}
		meta, digests, err := relBody(fields, rel, false)
		if err != nil {
			return r.relFailed(t.addr, local, "push: "+err.Error())
		}
		body, err := json.Marshal(map[string]any{"meta": meta})
		if err != nil { // coverage-ignore: canonical values always marshal
			return r.relFailed(t.addr, local, "push: "+err.Error())
		}
		if !r.relBudget() {
			return nil
		}
		ans, err := r.Send(ctx, http.MethodPatch, r.relPath(t, t.toID), body, etag, maxPushAnswer)
		if err != nil {
			return r.relSendFailed(t.addr, local, err)
		}
		switch {
		case ans.Status >= 200 && ans.Status < 300:
			maps.Copy(row.Base, digests)
			r.markSynced(row, local)
			pc.Updated++
			return nil
		case ans.Status == http.StatusPreconditionFailed:
			continue
		}
		r.relRefusedOrFailed(ans, t, local, digests)
		return nil
	}
	pc.Conflicts++
	r.issue(IssueConflict, t.addr, local, "the remote relation changed again during the push; compared again next run")
	return nil
}

// refreshRelation fetches the remote relation with its ETag, merges it into
// the local one as a pull does, and returns the local relation as the merge
// left it. It returns false when the relation cannot be pushed in this run.
func (r relSync) refreshRelation(
	ctx context.Context, t relTarget, row *ledgerRow,
) (rel *entity.Relation, etag string, ok bool, err error) {
	pc := r.rep.PushedRelations
	if !r.relBudget() {
		return nil, "", false, nil
	}
	re, etag, err := fetchRelationTagged(ctx, r, r.relPath(t, t.toID), t.fromID, t.rm.Source, t.toID)
	switch {
	case errors.Is(err, httpremote.ErrNotFound):
		pc.Deferred++
		return nil, "", false, nil
	case err != nil:
		return nil, "", false, r.relSendFailed(t.addr, row.Local, err)
	case etag == "":
		return nil, "", false, r.relFailed(t.addr, row.Local,
			"push: the remote sent no ETag for the relation, so it cannot be updated safely")
	}
	th, err := theirs(t.rm.Fields, re.Meta, re.Content, re.Redacted)
	if err != nil {
		pc.Failed++
		r.issue(IssueRejected, t.addr, row.Local, err.Error())
		return nil, "", false, nil
	}
	existing, err := r.d.Store.GetRelation(ctx, t.key)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, "", false, nil
	case err != nil:
		return nil, "", false, r.relFailed(t.addr, row.Local, "push: "+err.Error())
	}
	// The merge records the row with no source hash, so the next run
	// compares it again.
	src := sourceRow{th: th, updated: parseStamp(re.Updated)}
	if !r.mergeRelation(ctx, t.rm, row, t.addr, canonical.Digest{}, t.key, existing, src) {
		return nil, "", false, nil
	}
	if cur := r.l.get(reconclient.KindRelation, t.addr); cur != row || row.State != stateLinked {
		return nil, "", false, nil
	}
	rel, err = r.d.Store.GetRelation(ctx, t.key)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, "", false, nil
	case err != nil:
		return nil, "", false, r.relFailed(t.addr, row.Local, "push: "+err.Error())
	}
	return rel, etag, true, nil
}

// relBudget reports whether the run may send one more relation push
// request, counted against max_fetch and max_bytes like a fetch.
func (r relSync) relBudget() bool {
	if r.fetched >= r.m.Remote.maxFetch() || r.read >= r.m.Remote.maxBytes() {
		r.rep.PushedRelations.Deferred++
		r.rep.Waiting++
		return false
	}
	r.fetched++
	return true
}

// relSendFailed classifies an error of a relation push request as
// [pusher.pushFailed] does an entity's.
func (r relSync) relSendFailed(addr, local string, err error) error {
	var refused *reconclient.RefusedError
	switch {
	case errors.Is(err, httpremote.ErrDenied):
		return fmt.Errorf("remotesync: the remote refused the push's credential: %w", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.As(err, &refused):
		return fmt.Errorf("remotesync: the remote is busy; push stopped: %w", err)
	}
	return r.relFailed(addr, local, "push: "+err.Error())
}

// relFailed counts a relation whose push failed. It returns nil because
// one relation's failure does not end the run.
func (r relSync) relFailed(addr, local, msg string) error {
	r.rep.PushedRelations.Failed++
	r.issue(IssueFailed, addr, local, msg)
	return nil
}

// relRefusedOrFailed handles a relation push answer that is neither a
// success nor a 412. A 404 waits; a 400, 403, 409 or 422 holds the relation
// on the conflict list; anything else fails it.
func (r relSync) relRefusedOrFailed(ans httpremote.Answer, t relTarget, local string, digests map[string]string) {
	pc := r.rep.PushedRelations
	detail := reconclient.Sanitize(string(ans.Body))
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	switch ans.Status {
	case http.StatusNotFound:
		pc.Deferred++
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity:
		pc.Refused++
		addr := ""
		if row := r.l.get(reconclient.KindRelation, t.addr); row != nil && row.State == stateLinked {
			addr = t.addr
		}
		r.c.hold(&Conflict{
			Kind: KindRelation, Local: local, Type: t.rm.Local, Address: addr, Reason: ReasonRefused,
			Detail: detail, RefusedFields: digests, Cause: CauseRefused, Status: ans.Status,
		}, r.d.Now())
		r.issue(IssueRefused, t.addr, local, fmt.Sprintf("the remote refused the push (%d): %s; held on the conflict list",
			ans.Status, detail))
	default:
		pc.Failed++
		r.issue(IssueFailed, t.addr, local, fmt.Sprintf("push: the remote answered %d: %s", ans.Status, detail))
	}
}

// fetchRelationTagged reads one remote relation at path with its ETag,
// through p, and checks it is the one asked for.
func fetchRelationTagged(ctx context.Context, p Pusher, path, from, typ, to string) (remoteRelation, string, error) {
	ans, err := p.Send(ctx, http.MethodGet, path, nil, "", maxRelationBytes)
	switch {
	case err != nil:
		return remoteRelation{}, "", err
	case ans.Status == http.StatusNotFound:
		return remoteRelation{}, "", fmt.Errorf("%w: answered 404", httpremote.ErrNotFound)
	case ans.Status == http.StatusForbidden:
		return remoteRelation{}, "", fmt.Errorf("%w: answered 403", httpremote.ErrDenied)
	case ans.Status != http.StatusOK:
		return remoteRelation{}, "", fmt.Errorf("remotesync: the source answered %d: %s", ans.Status,
			reconclient.Sanitize(string(ans.Body)))
	}
	rel, err := checkRelation(ans.Body, from, typ, to)
	return rel, ans.ETag, err
}
