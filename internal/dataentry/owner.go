package dataentry

import (
	"context"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// ownerResolver answers which entity a row is shown as part of (TKT-QO14GB).
//
// An entity's owner is the source of its incoming `owning:` edge. It is
// served only when:
//
//   - the principal can read exactly one owning parent, and
//   - that parent has no readable owning parent of its own, and
//   - the parent is not the entity itself.
//
// Each test counts readable parents only. Counting hidden ones would make
// the presence or absence of an owner an oracle on hidden entities. The
// write path keeps ownership to one owner and one level, so the tests matter
// only for imported or hand-edited data, and they also rule out a
// redirect loop between two entities that own each other.
//
// The cost per page is fixed: two relation queries and two gated header
// batches, and none at all when no row's type can be owned.
type ownerResolver struct {
	meta  *metamodel.Metamodel
	store store.RelationReader
	// headers resolves ids to the readable header the request's world
	// serves; see [visibility.Resolver.ResolveIDsErr].
	headers func(ctx context.Context, ids []string) (map[string]store.EntityHeader, error)
	red     visibility.FieldRedactor
}

func newOwnerResolver(
	meta *metamodel.Metamodel, st store.RelationReader, vr visibleReader, red visibility.FieldRedactor,
) ownerResolver {
	return ownerResolver{meta: meta, store: st, headers: vr.servedIDsErr, red: red}
}

// appOwners is the resolver for a request served by a. A function rather
// than an App method: App is at its plimsoll method load line.
func appOwners(a *App) ownerResolver {
	return newOwnerResolver(a.Meta(), a.Services().Store, a.visibleReader, appRedactor(a))
}

// ownedTypes returns the entity types that are the target of some owning
// relation.
func ownedTypes(meta *metamodel.Metamodel) map[string]bool {
	if meta == nil {
		return nil
	}
	out := map[string]bool{}
	for name, def := range meta.Relations {
		if !metamodel.IsOwning(meta, name) {
			continue
		}
		for _, t := range def.To {
			out[t] = true
		}
	}
	return out
}

// soleOwningType returns the owning relation type when the schema declares
// exactly one.
func soleOwningType(meta *metamodel.Metamodel) (string, bool) {
	found := ""
	for name := range meta.Relations {
		if !metamodel.IsOwning(meta, name) {
			continue
		}
		if found != "" {
			return "", false
		}
		found = name
	}
	return found, found != ""
}

// resolve returns the owner of each row that has one, keyed by row id.
func (o ownerResolver) resolve(ctx context.Context, rows []*entityPkg.Entity) (map[string]*v1.EntityOwner, error) {
	none := map[string]*v1.EntityOwner{}
	owned := ownedTypes(o.meta)
	if len(owned) == 0 {
		return none, nil
	}
	ids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, e := range rows {
		if e != nil && owned[e.Type] && !seen[e.ID] {
			seen[e.ID] = true
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return none, nil
	}

	parents, err := o.readableParents(ctx, ids)
	if err != nil {
		return nil, err
	}
	candidate := map[string]parentEdge{}
	var parentIDs []string
	for child, ps := range parents {
		if len(ps) != 1 || ps[0].header.ID == child {
			continue
		}
		candidate[child] = ps[0]
		parentIDs = append(parentIDs, ps[0].header.ID)
	}
	if len(candidate) == 0 {
		return none, nil
	}

	// A parent that is itself shown as part of something is not an owner to
	// navigate to: the chain is irregular, and following it could loop.
	grand, err := o.readableParents(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*v1.EntityOwner, len(candidate))
	for child, p := range candidate {
		if len(grand[p.header.ID]) > 0 {
			continue
		}
		rh := visibility.RedactHeader(ctx, o.red, p.header)
		out[child] = &v1.EntityOwner{
			ID:       rh.ID,
			Type:     rh.Type,
			Title:    o.meta.DisplayTitle(rh.ID, rh.Type, rh.Properties),
			Relation: p.relation,
		}
	}
	return out, nil
}

// parentEdge is one readable owning parent and the relation that owns.
type parentEdge struct {
	header   store.EntityHeader
	relation string
}

// readableParents returns, for each id, its owning parents the principal may
// read. One relation query and one gated header batch for all ids.
func (o ownerResolver) readableParents(ctx context.Context, ids []string) (map[string][]parentEdge, error) {
	out := map[string][]parentEdge{}
	if len(ids) == 0 {
		return out, nil
	}
	type edge struct{ from, to, relType string }
	var edges []edge
	var fromIDs []string
	seen := map[string]bool{}
	q := store.RelationQuery{EntityIDs: ids, Direction: store.DirectionIncoming}
	// With one owning type, let the store filter; otherwise every incoming
	// edge is read and the owning ones kept below.
	if t, ok := soleOwningType(o.meta); ok {
		q.Type = t
	}
	for r, err := range o.store.ListRelations(ctx, q) {
		if err != nil {
			return nil, err
		}
		if !metamodel.IsOwning(o.meta, r.Type) {
			continue
		}
		edges = append(edges, edge{from: r.From, to: r.To, relType: r.Type})
		if !seen[r.From] {
			seen[r.From] = true
			fromIDs = append(fromIDs, r.From)
		}
	}
	if len(edges) == 0 {
		return out, nil
	}
	headers, err := o.headers(ctx, fromIDs)
	if err != nil {
		return nil, err
	}
	counted := map[[2]string]bool{}
	for _, e := range edges {
		h, ok := headers[e.from]
		// Two owning relation types between the same pair are one parent.
		if !ok || counted[[2]string{e.to, e.from}] {
			continue
		}
		counted[[2]string{e.to, e.from}] = true
		out[e.to] = append(out[e.to], parentEdge{header: h, relation: e.relType})
	}
	return out, nil
}

// serveOwners sets the owner of each row in data, which is parallel to rows.
// It reports false after writing the error response when the lookup failed.
func serveOwners(w http.ResponseWriter, r *http.Request, a *App, rows []*entityPkg.Entity, data []v1.Entity) bool {
	owners, err := appOwners(a).resolve(r.Context(), rows)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	setOwners(data, owners)
	return true
}

// setOwners copies owners onto the wire rows, which are parallel to rows.
func setOwners(data []v1.Entity, owners map[string]*v1.EntityOwner) {
	if len(owners) == 0 {
		return
	}
	for i := range data {
		data[i].Owner = owners[data[i].ID]
	}
}
