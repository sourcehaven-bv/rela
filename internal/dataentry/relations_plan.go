package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// edgeSlot names one edge of a relation type as the path entity sees it: the
// peer at the other end, and the edge's tail. An outgoing edge has one slot
// per peer. An incoming content-scoped edge belongs to one face of its
// source, the peer, so one peer may hold several slots: `POL-1@draft` and
// `POL-1@published` both citing the path entity are two edges.
type edgeSlot struct {
	peer string
	tail entity.Face
}

// edgeOp is one write a relation wrapper asks for: an upsert of the edge at
// slot (a create when existing is nil), or its removal.
type edgeOp struct {
	slot     edgeSlot
	ref      v1.ResourceIdentifier
	existing *entity.Relation
	remove   bool
}

// edgePlanner is the signature of edgeReader.plan, injected into the write
// handler and the affordance service.
type edgePlanner func(
	ctx context.Context, entityID string, tail entity.Face, canonical string, incoming bool,
	upd v1.RelationsUpdate, path string,
) ([]edgeOp, error)

// edgeReader reads the current edges a relation wrapper is matched against.
// The wiring site builds one from the App's read seams; it holds no state of
// its own.
type edgeReader struct {
	meta    func() *metamodel.Metamodel
	reader  entityReader
	visible visibleReader
}

// plan turns one relation wrapper into the edge writes it asks for. It
// is the one place a wrapper is matched against the current edges, shared by
// the affordance check and the reconciler, so the two cannot disagree on
// which edge a body names.
//
// The current set holds only the edges the principal can read
// ([visibleReader.readableRelations]): an edge the caller cannot see is never
// matched, so a `data` replace never deletes it and a `remove` cannot name
// it. tail is the path entity's face; it picks the outgoing edges of a
// content-scoped type, which belong to that face.
//
// A bare id for an incoming content edge names the edges that peer already
// has to the path entity. `data` and `add` keep all of them; `remove` needs
// exactly one, and answers face_required otherwise. A bare id with no edge
// yet is resolved as a single create resolves it ([visibility.Resolver.WriteTarget]).
func (e edgeReader) plan(
	ctx context.Context, entityID string, tail entity.Face, canonical string, incoming bool,
	upd v1.RelationsUpdate, path string,
) ([]edgeOp, error) {
	meta := e.meta()
	content := metamodel.IsContentScoped(meta, canonical)
	newTail := entity.ImplicitFace
	if !incoming && content {
		newTail = tail
	}
	current, err := e.currentEdges(ctx, entityID, newTail, canonical, incoming)
	if err != nil {
		return nil, &gateFaultError{err: err}
	}
	p := edgePlan{
		incoming: incoming, content: content, newTail: newTail, path: path,
		bySlot: map[edgeSlot]*entity.Relation{}, byPeer: map[string][]*entity.Relation{},
		resolve: func(id string) (entity.Face, error) { return e.resolveIncomingPeer(ctx, canonical, id, path) },
		named: func(id string, face entity.Face) error {
			return e.requireReadableFace(ctx, canonical, entity.FormatStateRef(id, face), id, face)
		},
	}
	for _, rel := range current {
		s := edgeSlot{peer: rel.To, tail: rel.FromFace}
		if incoming {
			s.peer = rel.From
		}
		p.bySlot[s] = rel
		p.byPeer[s.peer] = append(p.byPeer[s.peer], rel)
	}
	if upd.Delta {
		return p.delta(upd)
	}
	return p.replace(upd.Data)
}

// currentEdges returns the edges of canonical touching entityID that the
// principal can read. Outgoing edges are those tail owns (BUG-64MU2Q);
// incoming ones carry the peer's tail, so every face's are returned.
func (e edgeReader) currentEdges(
	ctx context.Context, entityID string, tail entity.Face, canonical string, incoming bool,
) ([]*entity.Relation, error) {
	var edges []*entity.Relation
	if incoming {
		edges = e.reader.incomingRelations(ctx, entityID)
	} else {
		edges = e.reader.outgoingRelations(ctx, entityID)
	}
	out := edges[:0:0]
	for _, edge := range edges {
		if edge.Type == canonical && (incoming || edge.FromFace == tail) {
			out = append(out, edge)
		}
	}
	return e.visible.readableRelations(ctx, out)
}

// resolveIncomingPeer is the face a new incoming content edge from the bare
// id takes. A peer that does not exist, or that the caller cannot read, is
// left at the implicit face for the manager to refuse as missing.
func (e edgeReader) resolveIncomingPeer(ctx context.Context, relType, id, path string) (entity.Face, error) {
	typ, err := e.visible.readableType(ctx, id)
	if err != nil {
		return "", &gateFaultError{err: err}
	}
	if typ == "" {
		return entity.ImplicitFace, nil
	}
	ref, ok, err := e.visible.resolver.WriteTarget(ctx, worldFromContext(ctx).visibility(), typ,
		entity.BareAddress(id))
	var amb *visibility.AmbiguousAddressError
	switch {
	case errors.As(err, &amb):
		return "", &structuralError{Code: "face_required", Path: path, Detail: amb.Error()}
	case err != nil:
		return "", &gateFaultError{err: err}
	case !ok:
		return "", danglingPeerError(relType, id)
	}
	return ref.Face, nil
}

// requireReadableFace refuses a new incoming content edge from a face the
// principal cannot read. Such a face is the same miss as one that does not
// exist: the error is the dangling-peer error a write to an absent id gets,
// and it names only the address the caller sent.
func (e edgeReader) requireReadableFace(ctx context.Context, relType, addr, id string, face entity.Face) error {
	typ, err := e.visible.readableType(ctx, id)
	if err != nil {
		return &gateFaultError{err: err}
	}
	if typ == "" {
		return danglingPeerError(relType, addr)
	}
	fam, ok, err := e.visible.family(ctx, typ, id)
	switch {
	case err != nil:
		return &gateFaultError{err: err}
	case !ok || !slices.Contains(fam.Faces, face):
		return danglingPeerError(relType, addr)
	}
	return nil
}

// edgePlan is the state of one edgeReader.plan call.
type edgePlan struct {
	incoming, content bool
	newTail           entity.Face
	path              string
	bySlot            map[edgeSlot]*entity.Relation
	byPeer            map[string][]*entity.Relation
	resolve           func(id string) (entity.Face, error)
	// named refuses a new edge from a face the principal cannot read.
	named func(id string, face entity.Face) error
}

// replace plans a `data` wrapper: upsert every listed edge, remove every
// current edge not listed.
func (p edgePlan) replace(refs []v1.ResourceIdentifier) ([]edgeOp, error) {
	ops, kept, err := p.upserts(refs, "data")
	if err != nil {
		return nil, err
	}
	for s, rel := range p.bySlot {
		if !kept[s] {
			ops = append(ops, edgeOp{slot: s, ref: v1.ResourceIdentifier{ID: s.peer}, existing: rel, remove: true})
		}
	}
	return ops, nil
}

// delta plans an `add`/`remove` wrapper. Removing an edge that does not
// exist is a no-op, so a retried request succeeds.
func (p edgePlan) delta(upd v1.RelationsUpdate) ([]edgeOp, error) {
	ops, added, err := p.upserts(upd.Add, "add")
	if err != nil {
		return nil, err
	}
	for i, ref := range upd.Remove {
		at := fmt.Sprintf("%s/remove/%d", p.path, i)
		slots, err := p.slots(ref, at, true)
		if err != nil {
			return nil, err
		}
		for _, s := range slots {
			if added[s] {
				return nil, &v1.WireError{
					Code: "shape_conflict", Path: at,
					Detail: fmt.Sprintf("edge %q is both added and removed", ref.ID),
				}
			}
			if rel := p.bySlot[s]; rel != nil {
				ops = append(ops, edgeOp{slot: s, ref: ref, existing: rel, remove: true})
			}
		}
	}
	return ops, nil
}

// upserts plans an upsert per listed edge, collapsing duplicates to one.
func (p edgePlan) upserts(refs []v1.ResourceIdentifier, key string) ([]edgeOp, map[edgeSlot]bool, error) {
	var ops []edgeOp
	at := map[edgeSlot]int{}
	for i, ref := range refs {
		slots, err := p.slots(ref, fmt.Sprintf("%s/%s/%d", p.path, key, i), false)
		if err != nil {
			return nil, nil, err
		}
		for _, s := range slots {
			op := edgeOp{slot: s, ref: ref, existing: p.bySlot[s]}
			if j, dup := at[s]; dup {
				ops[j] = op
				continue
			}
			at[s] = len(ops)
			ops = append(ops, op)
		}
	}
	listed := make(map[edgeSlot]bool, len(at))
	for s := range at {
		listed[s] = true
	}
	return ops, listed, nil
}

// slots is the edges one listed id names; see edgeReader.plan.
func (p edgePlan) slots(ref v1.ResourceIdentifier, at string, removing bool) ([]edgeSlot, error) {
	peer := peerAddress(ref.ID, p.incoming)
	id := peer.ID()
	if !p.incoming {
		return []edgeSlot{{peer: id, tail: p.newTail}}, nil
	}
	if !p.content {
		return []edgeSlot{{peer: id, tail: entity.ImplicitFace}}, nil
	}
	if named, ok := peer.Named(); ok {
		s := edgeSlot{peer: id, tail: named.Face}
		if !removing && p.bySlot[s] == nil {
			if err := p.named(id, named.Face); err != nil {
				return nil, err
			}
		}
		return []edgeSlot{s}, nil
	}
	existing := p.byPeer[id]
	switch {
	case len(existing) == 1, len(existing) > 1 && !removing:
		out := make([]edgeSlot, len(existing))
		for i, rel := range existing {
			out[i] = edgeSlot{peer: id, tail: rel.FromFace}
		}
		return out, nil
	case len(existing) > 1:
		return nil, &structuralError{
			Code: "face_required", Path: at + "/id",
			Detail: fmt.Sprintf("%s links here from more than one face; name the face, as %s",
				id, entity.FormatStateRef(id, existing[0].FromFace)),
		}
	case removing:
		return nil, nil
	}
	face, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return []edgeSlot{{peer: id, tail: face}}, nil
}

// edgeKeyOf is the stored key of the edge at s, seen from entityID.
func edgeKeyOf(entityID, relType string, s edgeSlot, incoming bool) entity.RelationKey {
	from, to := edgeEndpoints(entityID, s.peer, incoming)
	return entity.RelationKey{From: from, FromFace: s.tail, Type: relType, To: to}
}

// readableIncoming returns the incoming edges of id whose source the
// principal can read: at its tail face for a content-scoped edge, at some
// face otherwise ([visibleReader.readableRelations]). The read is ACL only,
// never narrowed by the world: the relations routes serve an edge editor,
// and a user manipulates the set they can see.
func readableIncoming(ctx context.Context, r entityReader, v visibleReader, id string) ([]*entity.Relation, error) {
	return v.readableRelations(ctx, r.incomingRelations(ctx, id))
}

// markIncomingFace adds `face` and `editable` to the wire row of an incoming
// edge whose tail is a face. Such an edge belongs to that face of its source,
// so a client addresses it as `ID@face`; editable reports whether the
// principal may remove it. The server re-authorizes every write.
func (a *App) markIncomingFace(
	ctx context.Context, row map[string]any, pathEntity *entity.Entity, edge *entity.Relation,
) {
	if edge.FromFace.IsImplicit() {
		return
	}
	row["face"] = string(edge.FromFace)
	row["editable"] = false
	req := translateRelationDelete(a.Meta(), edge.Type, a.reader.entityType(ctx, edge.From), edge.From, edge.FromFace)
	if !a.affordances.acl().AuthorizeWrite(ctx, req).Allow {
		return
	}
	sources, err := a.affordances.relationSources(ctx, pathEntity,
		entity.Ref{ID: edge.From, Face: edge.FromFace}, string(DirectionIncoming), edge.Type)
	if err != nil {
		return
	}
	_, denial := a.affordances.relationOpDenial(ctx, sources, edge.Type, RelationOpRemove)
	row["editable"] = denial == nil
}
