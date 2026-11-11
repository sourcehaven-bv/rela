package remotesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// The push of a remote with push (DEC-2NFLAHB) runs after the pull, so it
// sees the local rows as the pull left them. Per shared local row:
//
//   - a linked row sends the mapped fields whose local value moved off the
//     merge base, by PATCH with If-Match on the ETag the run last saw. A
//     412 means the remote changed the row: the run fetches it, merges it as
//     a pull does, and tries once more. A second 412 leaves the row for the
//     next run. A 403, 409 or 422 holds the row on the conflict list.
//   - a row the ledger does not know and whose ref is empty is created on
//     the remote by POST, without an id: the remote picks it, as its API
//     does for any caller. The ledger then maps the local id to the remote
//     id, and the local ref is set to the remote id, so a run whose ledger
//     was lost links the two again instead of creating a second copy.
//
// A pushed value becomes the field's merge base, so the next pull of the
// row finds both sides at the base and writes nothing (the echo stop).
// Fields held on the conflict list are not pushed.

// maxPushAnswer caps the body of a push's answer.
const maxPushAnswer = maxEntityBytes

// pusher runs the push phase of a run. It shares the run's state: the
// ledger, the conflict list and the report.
type pusher struct{ *runner }

// push sends the local changes of every mapped type.
func (r pusher) push(ctx context.Context) error {
	byLocal := map[string]*ledgerRow{}
	for _, row := range r.l.rows {
		if row.Kind == "E" && row.Local != "" {
			byLocal[row.Local] = row
		}
	}
	// seen becomes pushSeen only when the push finished: a push that
	// stopped early did not look at every row, so it cannot tell which
	// rows left the share scope.
	seen := map[string]bool{}
	for _, src := range sortedKeys(r.m.Entities) {
		em := r.m.Entities[src]
		rows, err := r.localRows(ctx, em, seen)
		if err != nil {
			return err
		}
		for _, e := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			seen[e.ID] = true
			if err := r.pushEntity(ctx, em, e, byLocal[e.ID]); err != nil {
				return err
			}
		}
	}
	r.pushSeen = seen
	return nil
}

// localRows reads the local rows of em's type that the principal on ctx
// may read and the share condition selects, as that principal sees them:
// the push sends no row and no field the principal cannot read, as the
// pull merges into none (TKT-A6U657Q). The refs are listed in full before
// any read through the gate, so nothing runs inside the store's iteration.
// A row the push skips without judging it, because the principal cannot
// read it or the condition cannot be evaluated, is marked in seen, so its
// conflict list entries stay.
func (r pusher) localRows(ctx context.Context, em *EntityMapping, seen map[string]bool) ([]*entity.Entity, error) {
	var refs []entity.Ref
	for e, err := range r.d.Store.ListEntities(ctx, store.EntityQuery{Type: em.Local, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, fmt.Errorf("remotesync: list local %s rows: %w", em.Local, err)
		}
		refs = append(refs, e.Ref())
	}
	var out []*entity.Entity
	unjudged, unchecked := 0, 0
	for _, ref := range refs {
		e, err := r.d.Gate.GetAddress(ctx, ref.String())
		switch {
		case errors.Is(err, store.ErrNotFound):
			// Not readable: not pushed, and not named in the report.
			seen[ref.ID] = true
			continue
		case err != nil:
			r.d.Logger.Error("remote push: check the local row", "run_id", r.d.RunID, "local", ref.String(), "err", err)
			unchecked++
			seen[ref.ID] = true
			continue
		}
		if em.Share != nil {
			ok, err := r.m.eval.Matches(ctx, em.Share, em.Local, e.ID, e.Properties)
			if err != nil {
				// A condition that cannot be evaluated for a row does not
				// share it.
				unjudged++
				seen[e.ID] = true
				continue
			}
			if !ok {
				continue
			}
		}
		out = append(out, e)
	}
	if unchecked > 0 {
		r.rep.Pushed.Failed += unchecked
		r.issue(IssueFailed, "", "", fmt.Sprintf("could not check whether the pusher can read %d %s rows; "+
			"they were not pushed", unchecked, em.Local))
	}
	if unjudged > 0 {
		r.issue(IssueFailed, "", "", fmt.Sprintf("share of %s could not be evaluated for %d rows; they were not pushed",
			em.Source, unjudged))
	}
	return out, nil
}

// pushEntity pushes one shared local row.
func (r pusher) pushEntity(ctx context.Context, em *EntityMapping, e *entity.Entity, row *ledgerRow) error {
	if row == nil {
		if id, _ := metamodel.ExternalRefID(e.Properties[em.Ref]); id != "" {
			// The row has a remote counterpart the ledger does not know;
			// the pull links it again when the remote lists it.
			return nil
		}
		return r.pushCreate(ctx, em, e)
	}
	if row.State != stateLinked || r.rowHeld(em, e) {
		return nil
	}
	if id, _ := metamodel.ExternalRefID(e.Properties[em.Ref]); id == "" && !r.o.DryRun {
		// A create whose ref write failed: set it now, so a lost ledger
		// still links the two rows.
		if rid, ok := addressID(row.Address); ok {
			r.setRef(ctx, em, e, row.Address, rid)
		}
	}
	return r.pushUpdate(ctx, em, e, row)
}

// rowHeld reports whether the whole local row waits on the conflict list.
// A refused push stops waiting once a local value it tried to send changed:
// the next push sends the new values, and the remote judges them again.
func (r pusher) rowHeld(em *EntityMapping, e *entity.Entity) bool {
	held, ok := r.c.entries[conflictKey(e.ID, "")]
	if !ok {
		return false
	}
	if held.Reason != ReasonRefused {
		return true
	}
	for _, f := range pushFields(em, e) {
		d, tried := held.RefusedFields[f.baseKey()]
		if tried && syncmerge.Digest(f.Local, oursValue(f.Local, e)) != d {
			r.c.drop(e.ID, "")
			return false
		}
	}
	return true
}

// changed returns the mapped fields of e whose value moved off row's merge
// base and are not held. A field without a base is left alone: the pull has
// not agreed a value for it yet.
func (r pusher) changed(em *EntityMapping, e *entity.Entity, row *ledgerRow) []FieldMapping {
	var out []FieldMapping
	for _, f := range pushFields(em, e) {
		b, ok := row.Base[f.baseKey()]
		if !ok {
			continue
		}
		d := syncmerge.Digest(f.Local, oursValue(f.Local, e))
		if held, isHeld := r.c.entries[conflictKey(e.ID, f.Local.Name)]; isHeld && held.RemoteDigest == d {
			// The local side now has the remote's value: the two agree, and
			// the agreed value is the new base.
			r.c.drop(e.ID, f.Local.Name)
			row.Base[f.baseKey()] = d
			continue
		}
		if r.c.blocked(e.ID, f.Local.Name) {
			continue
		}
		if d != b {
			out = append(out, f)
		}
	}
	return out
}

// pushBody is the JSON of a create or update: the mapped fields under their
// remote names. A cleared field is unset on an update and left out of a
// create.
func pushBody(
	fields []FieldMapping, e *entity.Entity, create bool,
) (body []byte, digests map[string]string, err error) {
	props := map[string]any{}
	var unset []string
	var content *string
	digests = map[string]string{}
	for _, f := range fields {
		var v any
		v, err = syncmerge.Canonical(f.Local, oursValue(f.Local, e))
		if err != nil {
			return nil, nil, err
		}
		digests[f.baseKey()] = syncmerge.Digest(f.Local, v)
		switch {
		case f.toContent():
			s, _ := v.(string)
			content = &s
		case v == nil && !create:
			unset = append(unset, f.Remote)
		case v != nil:
			props[f.Remote] = v
		}
	}
	doc := map[string]any{"properties": props}
	if len(unset) > 0 {
		doc["properties_unset"] = unset
	}
	if content != nil && (!create || *content != "") {
		doc["content"] = *content
	}
	body, err = json.Marshal(doc)
	return body, digests, err
}

// pushUpdate sends row's changed fields by PATCH with If-Match.
func (r pusher) pushUpdate(ctx context.Context, em *EntityMapping, e *entity.Entity, row *ledgerRow) error {
	pc := r.rep.Pushed
	id, _ := addressID(row.Address)
	fields := r.changed(em, e, row)
	if len(fields) == 0 {
		return nil
	}
	if r.o.DryRun {
		pc.Updated++
		return nil
	}
	for attempt := range maxAttempts {
		if row.ETag == "" || attempt > 0 {
			// The ETag is unknown, or stale after a 412: fetch the row and
			// merge it first.
			var ok bool
			var err error
			if e, ok, err = r.refresh(ctx, em, row, id); err != nil || !ok {
				return err
			}
			if fields = r.changed(em, e, row); len(fields) == 0 {
				return nil
			}
		}
		if !r.pushBudget() {
			return nil
		}
		body, digests, err := pushBody(fields, e, false)
		if err != nil {
			return r.rowFailed(row.Address, e.ID, "push: "+err.Error())
		}
		ans, err := r.Send(ctx, http.MethodPatch, entityPath(r.plurals[em.Source], id), body, row.ETag, maxPushAnswer)
		if err != nil {
			return r.pushFailed(row.Address, e.ID, err)
		}
		switch {
		case ans.Status >= 200 && ans.Status < 300:
			maps.Copy(row.Base, digests)
			row.ETag = ans.ETag
			pc.Updated++
			return nil
		case ans.Status == http.StatusPreconditionFailed:
			continue
		}
		r.pushRefusedOrFailed(ans, em, e, row.Address, digests)
		return nil
	}
	pc.Conflicts++
	r.issue(IssueConflict, row.Address, e.ID, "the remote row changed again during the push; compared again next run")
	return nil
}

// refresh fetches row's remote entity with its ETag and merges it into the
// local row, as a pull does. It returns the local row as the merge left it,
// and false when the row cannot be pushed in this run.
func (r pusher) refresh(
	ctx context.Context, em *EntityMapping, row *ledgerRow, id string,
) (*entity.Entity, bool, error) {
	pc := r.rep.Pushed
	if !r.pushBudget() {
		return nil, false, nil
	}
	re, etag, err := fetchTagged(ctx, r, r.plurals[em.Source], em.Source, id)
	switch {
	case errors.Is(err, httpremote.ErrNotFound):
		pc.Deferred++
		return nil, false, nil
	case err != nil:
		return nil, false, r.pushFailed(row.Address, row.Local, err)
	}
	th, err := theirs(em.Fields, re.Properties, re.Content, re.Redacted)
	if err != nil {
		pc.Failed++
		r.issue(IssueRejected, row.Address, row.Local, err.Error())
		return nil, false, nil
	}
	// The merge records the row with no source hash: the run does not know
	// the view hash of what it fetched, so the next run compares it again.
	r.mergeFetched(ctx, em, row, row.Address, id, canonical.Digest{}, etag, th)
	if cur := r.l.get(reconclient.KindEntity, row.Address); cur != row || row.State != stateLinked {
		return nil, false, nil
	}
	if row.ETag == "" {
		return nil, false, r.rowFailed(row.Address, row.Local, "push: the remote sent no ETag for the row, so it cannot be updated safely")
	}
	e, err := r.d.Gate.GetAddress(ctx, row.localRef().String())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, false, nil
	case err != nil:
		r.d.Logger.Error("remote push: read the local row", "run_id", r.d.RunID, "address", row.Address, "err", err)
		return nil, false, r.rowFailed(row.Address, "", "push: could not read the local row")
	}
	return e, true, nil
}

// pushFields returns the mapped fields of e the push may send: those not
// withheld from the principal on ctx when e was read through the gate.
func pushFields(em *EntityMapping, e *entity.Entity) []FieldMapping {
	if len(e.Redacted) == 0 {
		return em.Fields
	}
	out := make([]FieldMapping, 0, len(em.Fields))
	for _, f := range em.Fields {
		if f.Local.Kind == syncmerge.KindContent || !e.IsRedacted(f.Local.Name) {
			out = append(out, f)
		}
	}
	return out
}

// pushCreate creates a new shared local row on the remote.
func (r pusher) pushCreate(ctx context.Context, em *EntityMapping, e *entity.Entity) error {
	pc := r.rep.Pushed
	if r.rowHeld(em, e) {
		return nil
	}
	if r.o.DryRun {
		pc.Created++
		return nil
	}
	if !r.pushBudget() {
		return nil
	}
	body, digests, err := pushBody(pushFields(em, e), e, true)
	if err != nil {
		return r.rowFailed("", e.ID, "push: "+err.Error())
	}
	ans, err := r.Send(ctx, http.MethodPost, r.plurals[em.Source], body, "", maxPushAnswer)
	if err != nil {
		// Any failure after the request may have left, a cancelled run
		// and a busy answer included, holds the row: the remote may have
		// made it.
		if !notSent(err) {
			r.unknownCreate(em, e, digests, err.Error())
		}
		return r.pushFailed("", e.ID, err)
	}
	switch {
	case ans.Status >= http.StatusInternalServerError:
		return r.uncertainCreate(em, e, digests, fmt.Sprintf("%d %s", ans.Status, reconclient.Sanitize(string(ans.Body))))
	case ans.Status < 200 || ans.Status >= 300:
		r.pushRefusedOrFailed(ans, em, e, "", digests)
		return nil
	}
	var created struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	decodeErr := decode(ans.Body, &created)
	if ans.Status != http.StatusCreated && ans.Status != http.StatusOK || decodeErr != nil ||
		created.Type != em.Source || entity.ValidateID(created.ID) != nil {

		return r.uncertainCreate(em, e, digests, fmt.Sprintf("%d with an answer that does not name a new %s",
			ans.Status, em.Source))
	}
	addr := em.Source + "/" + created.ID
	if r.l.get(reconclient.KindEntity, addr) != nil {
		// The remote named a row the ledger already maps; only the driver
		// writes the mapping, so it is not linked to a second local row.
		return r.uncertainCreate(em, e, digests, "the remote answered with "+addr+", which is already linked")
	}
	// A pushed row counts as adopted: it was here before the remote had
	// it, so on_gone never deletes it.
	row := &ledgerRow{
		Kind: "E", Type: em.Source, Address: addr, Local: e.ID, Face: e.Face, State: stateLinked,
		Base: digests, ETag: ans.ETag, Adopted: true,
	}
	r.l.put(row)
	r.byID[created.ID] = row
	pc.Created++
	r.setRef(ctx, em, e, addr, created.ID)
	return nil
}

// setRef sets e's ref to the remote row id, as on a pulled row. It is what
// links the two again if the ledger is lost. A failed write fails the row;
// the next push tries again.
func (r pusher) setRef(ctx context.Context, em *EntityMapping, e *entity.Entity, addr, id string) {
	if _, err := r.d.Writer.PatchEntity(ctx, e.ID, entity.Patch{
		Properties:        map[string]any{em.Ref: metamodel.ExternalRefValue{ID: id}.Map()},
		WriteExternalRefs: true,
	}); err != nil {
		_ = r.rowFailed(addr, e.ID, "pushed, but the local ref was not set: "+err.Error())
	}
}

// notSent reports whether a push request's error proves the remote did not
// act on it: the remote refused the credential, or no connection was made.
func notSent(err error) bool {
	var op *net.OpError
	return errors.Is(err, httpremote.ErrDenied) || errors.As(err, &op) && op.Op == "dial"
}

// uncertainCreate fails a create whose outcome is unknown and holds it.
func (r pusher) uncertainCreate(em *EntityMapping, e *entity.Entity, digests map[string]string, detail string) error {
	r.unknownCreate(em, e, digests, detail)
	return r.rowFailed("", e.ID, "push: the create may have reached the remote: "+detail)
}

// unknownCreate holds a row whose create failed in a way that does not say
// whether the remote made the row: a broken connection or a server error.
// Sending it again could make a second copy, so a person checks the remote
// first and resolves the entry to the local side to send it again.
func (r pusher) unknownCreate(em *EntityMapping, e *entity.Entity, digests map[string]string, detail string) {
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	r.c.hold(&Conflict{
		Local: e.ID, Type: em.Local, Reason: ReasonUncertain, RefusedFields: digests,
		Detail: "the create may have reached the remote (" + detail + "); check the remote for the row " +
			"before you resolve this to local",
	}, r.d.Now())
}

// pushBudget reports whether the run may send one more push request,
// counted against max_fetch and max_bytes like a fetch.
func (r pusher) pushBudget() bool {
	if r.fetched >= r.m.Remote.maxFetch() || r.read >= r.m.Remote.maxBytes() {
		r.rep.Pushed.Deferred++
		r.rep.Waiting++
		return false
	}
	r.fetched++
	return true
}

// pushFailed classifies an error of a push request: a credential the remote
// does not accept, a cancelled run and a rate limit end the run; anything
// else fails the row.
func (r pusher) pushFailed(addr, local string, err error) error {
	var refused *reconclient.RefusedError
	switch {
	case errors.Is(err, httpremote.ErrDenied):
		return fmt.Errorf("remotesync: the remote refused the push's credential: %w", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.As(err, &refused):
		return fmt.Errorf("remotesync: the remote is busy; push stopped: %w", err)
	}
	return r.rowFailed(addr, local, "push: "+err.Error())
}

// rowFailed counts a row whose push failed and reports it as an issue. It
// returns nil because one row's failure does not end the run.
func (r pusher) rowFailed(addr, local, msg string) error {
	r.rep.Pushed.Failed++
	r.issue(IssueFailed, addr, local, msg)
	return nil
}

// pushRefusedOrFailed handles a push answer that is neither a success nor
// a 412. A 404 waits: the diff of the next run finds the row gone. A 400,
// 403, 409 or 422 is the remote's refusal and holds the row; anything else
// fails the row.
func (r pusher) pushRefusedOrFailed(
	ans httpremote.Answer, em *EntityMapping, e *entity.Entity, addr string, digests map[string]string,
) {
	pc := r.rep.Pushed
	detail := reconclient.Sanitize(string(ans.Body))
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	switch ans.Status {
	case http.StatusNotFound:
		pc.Deferred++
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity:
		pc.Refused++
		r.c.hold(&Conflict{
			Local: e.ID, Type: em.Local, Address: addr, Reason: ReasonRefused,
			Detail: fmt.Sprintf("%d %s", ans.Status, detail), RefusedFields: digests,
		}, r.d.Now())
		r.issue(IssueRefused, addr, e.ID, fmt.Sprintf("the remote refused the push (%d): %s; held on the conflict list",
			ans.Status, detail))
	default:
		pc.Failed++
		r.issue(IssueFailed, addr, e.ID, fmt.Sprintf("push: the remote answered %d: %s", ans.Status, detail))
	}
}

// noteConflicts holds each field of mg in conflict on the conflict list and
// drops the entries of the fields that agree again.
func (r *runner) noteConflicts(em *EntityMapping, local, addr string, mg merged) {
	for _, f := range mg.agreed {
		r.c.drop(local, f.Local.Name)
	}
	for _, fc := range mg.conflicts {
		v, omitted := heldValue(fc.theirs)
		r.c.hold(&Conflict{
			Local: local, Type: em.Local, Address: addr, Field: fc.f.Local.Name, RemoteField: fc.f.Remote,
			Reason: ReasonBothChanged, RemoteValue: v, RemoteValueOmitted: omitted,
			LocalDigest: fc.ours, RemoteDigest: fc.theirsD,
		}, r.d.Now())
	}
}

// forgetConflicts drops the entries whose row is gone: an entry of a remote
// row the ledger no longer links, and a refused create of a local row the
// push no longer shares.
func (r *runner) forgetConflicts() {
	linked := map[string]bool{}
	for _, row := range r.l.rows {
		if row.Kind == "E" && row.State == stateLinked {
			linked[row.Local] = true
		}
	}
	for k, e := range r.c.entries {
		switch {
		case e.Address != "" && !linked[e.Local]:
			delete(r.c.entries, k)
		case e.Address == "" && r.pushSeen != nil && !r.pushSeen[e.Local]:
			delete(r.c.entries, k)
		case e.Address == "" && linked[e.Local]:
			delete(r.c.entries, k)
		}
	}
}

// addressID returns the id part of an entity address "type/id".
func addressID(addr string) (string, bool) {
	_, id, ok := strings.Cut(addr, "/")
	return id, ok
}
