package visibility

import (
	"context"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TaggedEntities lists the entity faces with a version tagged name, of
// entityType when it is not empty, that the caller may know of. A row is kept
// on the rules of the history reads:
//
//   - A live row is kept when the caller may read that face now, exactly as
//     [ScriptReader.GetAddress] of `ID@face` would serve it.
//   - A deleted row has no live row to gate, so it needs the global
//     acl.PermHistoryRead and the face half of the read grant for its type.
//     When other faces of the id are still live, they must be of the same
//     type and the caller must be able to read one of them. When the deleted
//     face itself lives again, the caller must be able to read that new row.
//     This is the rule the data-entry history endpoint applies.
//
// Every other row is left out, so a hidden entity is indistinguishable from
// one that was never tagged. A denied world lists nothing. A store that keeps
// no history or no tags answers [store.ErrHistoryUnsupported].
//
// The cost does not depend on the row count: one store query for the tags,
// one header query for their ids, and one gate probe per stored type.
func (s *ScriptReader) TaggedEntities(
	ctx context.Context, name store.VersionTagName, entityType string,
) ([]store.TaggedEntity, error) {
	if s.history == nil || s.tags == nil {
		return nil, store.ErrHistoryUnsupported
	}
	ctx = s.bind(ctx)
	rows, err := s.tags.TaggedEntities(ctx, name, entityType)
	if err != nil {
		return nil, err
	}
	return s.res.gateTagged(ctx, worldIn(ctx, s.world), rows)
}

// TaggedEntities is [ScriptReader.TaggedEntities] without the gate: every row
// is listed, deleted ones included.
func (r *UnrestrictedReader) TaggedEntities(
	ctx context.Context, name store.VersionTagName, entityType string,
) ([]store.TaggedEntity, error) {
	if r.history == nil || r.tags == nil {
		return nil, store.ErrHistoryUnsupported
	}
	return r.tags.TaggedEntities(ctx, name, entityType)
}

// TaggedEntities denies, like every DenyReader read: nothing is listed.
func (DenyReader) TaggedEntities(context.Context, store.VersionTagName, string) ([]store.TaggedEntity, error) {
	return nil, nil
}

// gateTagged keeps the rows of [ScriptReader.TaggedEntities]. A failed header
// read is returned; a gate error hides that type's rows and is logged, as in
// [Resolver.ResolveHeadersErr]. An unset world is a wiring bug and is
// returned.
func (r *Resolver) gateTagged(ctx context.Context, w World, rows []store.TaggedEntity) ([]store.TaggedEntity, error) {
	if !w.denied && !w.scope.IsSet() {
		return nil, fmt.Errorf("%w: visibility: TaggedEntities with an unset world (use WorldOf)",
			store.ErrInvalidQuery)
	}
	if w.denied || len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.Ref.ID
	}
	stored, err := r.storedHeaders(ctx, store.EntityQuery{IDs: distinctIDs(ids), Faces: store.AllFaces()})
	if err != nil {
		return nil, err
	}
	readable, err := r.gateHeaders(ctx, stored, func(typ string, err error) error {
		warnGate("tagged", typ, "", err)
		return nil
	})
	if err != nil {
		return nil, err
	}
	g := deletedRowGate{gate: r.gate, stored: stored, readable: readable}
	out := make([]store.TaggedEntity, 0, len(rows))
	for _, row := range rows {
		if row.Deleted {
			if g.admits(ctx, row) {
				out = append(out, row)
			}
			continue
		}
		if h, ok := readable[row.Ref.ID][row.Ref.Face]; ok && h.Type == row.Type {
			out = append(out, row)
		}
	}
	return out, nil
}

// deletedRowGate decides the deleted rows of one Resolver.gateTagged call.
// The permission and the face grants are asked once per call and per type.
type deletedRowGate struct {
	gate     RowGate
	stored   map[string][]store.EntityHeader
	readable map[string]map[entity.Face]store.EntityHeader

	asked   bool
	history bool
	faces   map[string]map[entity.Face]bool
}

// admits applies the deleted-face rule to row.
func (g *deletedRowGate) admits(ctx context.Context, row store.TaggedEntity) bool {
	if !g.holdsHistoryRead(ctx) || !g.faceAllowed(ctx, row.Type, row.Ref.Face) {
		return false
	}
	live := g.stored[row.Ref.ID]
	if len(live) == 0 {
		return true
	}
	typ, single := singleType(live)
	if !single || typ != row.Type {
		return false
	}
	if slices.ContainsFunc(live, func(h store.EntityHeader) bool { return h.Face == row.Ref.Face }) {
		// The face lives again. Its history is a live read, so the new row
		// itself must be readable, as the history endpoint requires.
		_, ok := g.readable[row.Ref.ID][row.Ref.Face]
		return ok
	}
	return len(g.readable[row.Ref.ID]) > 0
}

func (g *deletedRowGate) holdsHistoryRead(ctx context.Context) bool {
	if !g.asked {
		g.asked = true
		if ph, ok := g.gate.(PermissionHolder); ok {
			g.history = ph.HoldsPermission(ctx, acl.PermHistoryRead)
		}
	}
	return g.history
}

func (g *deletedRowGate) faceAllowed(ctx context.Context, typ string, face entity.Face) bool {
	if g.faces == nil {
		g.faces = make(map[string]map[entity.Face]bool)
	}
	byFace := g.faces[typ]
	if byFace == nil {
		byFace = make(map[entity.Face]bool)
		g.faces[typ] = byFace
	}
	allowed, ok := byFace[face]
	if !ok {
		allowed = FaceAllowed(ctx, g.gate, typ, face)
		byFace[face] = allowed
	}
	return allowed
}
