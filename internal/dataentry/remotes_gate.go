package dataentry

import (
	"context"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// remoteGate is what the caller may read of a batch of remote rows: the
// served, redacted row of each readable local entity, and whether each
// relation's endpoints are readable. [remotesHandler.gate] builds it in one
// pass per request; nothing caches it across requests or principals.
type remoteGate struct {
	aff   affordanceService
	rows  map[string]*entityPkg.Entity
	relOK map[RemoteRelationKey]bool
}

// remoteVerdict is the answer to one action on one entry.
type remoteVerdict int

const (
	remoteAllowed remoteVerdict = iota
	// remoteWrongState: the entry's reason does not admit the action.
	remoteWrongState
	// remoteForbidden: the caller lacks the edit right the action needs.
	remoteForbidden
)

// gate reads, in one batch, every local row entries and linked name: one
// header resolution in the request's world for every entity id and
// relation endpoint, and one endpoint check for every relation. It returns
// ctx primed for the field verdicts of the served rows (read paths only).
func (h *remotesHandler) gate(
	ctx context.Context, entries []RemoteEntry, linked []RemoteRow,
) (context.Context, remoteGate, error) {
	g := remoteGate{aff: h.app.affordances, rows: map[string]*entityPkg.Entity{}, relOK: map[RemoteRelationKey]bool{}}
	var ids []string
	var keys []RemoteRelationKey
	add := func(row RemoteRow) {
		if row.Kind == remoteKindRelation {
			ids = append(ids, row.Relation.From, row.Relation.To)
			keys = append(keys, row.Relation)
			return
		}
		ids = append(ids, row.Local)
	}
	for _, e := range entries {
		add(e.RemoteRow)
	}
	for _, row := range linked {
		add(row)
	}
	if len(ids) == 0 {
		return ctx, g, nil
	}
	resolver := h.app.visibleReader.resolver
	served, err := resolver.ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)
	if err != nil {
		return ctx, g, err
	}
	rows := make([]*entityPkg.Entity, 0, len(served))
	for id, header := range served {
		row := headerEntity(header)
		g.rows[id] = row
		rows = append(rows, row)
	}
	if len(keys) > 0 {
		rels := make([]*entityPkg.Relation, len(keys))
		for i, k := range keys {
			rels[i] = &entityPkg.Relation{From: k.From, FromFace: entityPkg.Face(k.FromFace), Type: k.Type, To: k.To}
		}
		ok, err := resolver.EndpointsReadableErr(ctx, rels)
		if err != nil {
			return ctx, g, err
		}
		for i, k := range keys {
			g.relOK[k] = ok[i]
		}
	}
	return primeVerdicts(ctx, g.aff.resolver(), rows), g, nil
}

// rowVisible reports whether the caller may read row.
func (g remoteGate) rowVisible(row RemoteRow) bool {
	if row.Kind == remoteKindRelation {
		return g.relOK[row.Relation]
	}
	e := g.rows[row.Local]
	return e != nil && e.Type == row.Type
}

// entryVisible reports whether the caller may see e: its row, and its
// field when it names one. A relation field is judged by the relation's
// `visible:` grants on its source row, and is hidden when that row cannot
// be read.
func (g remoteGate) entryVisible(ctx context.Context, e RemoteEntry) bool {
	if !g.rowVisible(e.RemoteRow) {
		return false
	}
	if e.Field == "" {
		return true
	}
	if e.Kind != remoteKindRelation {
		return g.aff.fieldVerdicts(ctx, g.rows[e.Local]).IsVisible(e.Field)
	}
	from := g.rows[e.Relation.From]
	if from == nil {
		return false
	}
	rv, ok := g.aff.resolver().(RelationVisibilityResolver)
	if !ok {
		return true
	}
	visible, judged := rv.RelationFieldVerdicts(ctx, from, e.Relation.Type, []string{e.Field})[e.Field]
	return !judged || visible
}

// verdict answers whether the caller may take action on the visible entry
// e. `_actions` and the POST routes both ask it, so they cannot disagree.
//
// Taking the local side and dismissing change only the conflict list and
// the ledger; remote:sync and read access suffice. Taking the remote side
// makes the next pull write the remote values into the local row, so it
// also needs the caller's edit right on the row and on every field the
// entry names. Linking does the same: the next pull takes the linked remote
// row's value for every field the create tried to send.
func (h *remotesHandler) verdict(ctx context.Context, g remoteGate, e RemoteEntry, action string) remoteVerdict {
	switch action {
	case remoteActionResolveLocal:
		return stateVerdict(e.Reason != remoteReasonSupersede)
	case remoteActionDismiss:
		return stateVerdict(e.Reason == remoteReasonSupersede)
	case remoteActionLink:
		if e.Kind != "" || e.Field != "" || e.Reason != remoteReasonUncertain {
			return remoteWrongState
		}
		if h.mayWriteRemoteValue(ctx, g, e) {
			return remoteAllowed
		}
		return remoteForbidden
	case remoteActionResolveRemote:
		if e.Reason == remoteReasonSupersede || e.Reason != remoteReasonBoth && e.Address == "" {
			return remoteWrongState
		}
		if h.mayWriteRemoteValue(ctx, g, e) {
			return remoteAllowed
		}
		return remoteForbidden
	}
	return remoteWrongState
}

func stateVerdict(ok bool) remoteVerdict {
	if ok {
		return remoteAllowed
	}
	return remoteWrongState
}

// mayWriteRemoteValue reports whether the caller may write the fields e
// names on its local row: the update the next pull makes when e is
// resolved to the remote side, asked as the write path asks it.
func (h *remotesHandler) mayWriteRemoteValue(ctx context.Context, g remoteGate, e RemoteEntry) bool {
	fields := e.Fields
	if e.Field != "" {
		fields = []string{e.Field}
	}
	if e.Kind == remoteKindRelation {
		from := g.rows[e.Relation.From]
		if from == nil {
			return false
		}
		req := translateRelationWrite(h.app.Meta(), e.Relation.Type, from.Type, from.ID,
			entityPkg.Face(e.Relation.FromFace))
		if !g.aff.acl().AuthorizeWrite(ctx, req).Allow {
			return false
		}
		rv, ok := g.aff.outgoingRelationVerdicts(ctx, from).Types[e.Relation.Type]
		for _, f := range fields {
			if ok && !rv.MetaWritable(f) {
				return false
			}
		}
		return true
	}
	row := g.rows[e.Local]
	if row == nil || !g.aff.acl().AuthorizeWrite(ctx, translateVerb("update", row.Type, row.ID, row.Face)).Allow {
		return false
	}
	v := g.aff.fieldVerdicts(ctx, row)
	for _, f := range fields {
		if !v.IsVisible(f) || !v.IsWritable(f) {
			return false
		}
	}
	return true
}
