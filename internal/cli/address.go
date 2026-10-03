package cli

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// errFaceRequired is returned when a bare id names an entity that the world
// resolves to no face, such as a faced entity whose faces the default world
// does not serve. The message names the entity's faces so the operator can
// pick one.
var errFaceRequired = errors.New("address one face")

// addressLoader is the raw read [readAddress] needs. Satisfied by
// store.Store.
type addressLoader interface {
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// readAddress reads the row an entity address names, as the operator: the
// CLI applies no ACL, so it reads through the allow-all resolver.
//
// `ID@face` reads that face. A bare id resolves in world, the default world
// (readServices.World): the first face in it that the entity has. When the
// default world serves none of the entity's faces, the read fails with
// [errFaceRequired] and a message naming its faces, as `rela attach` does.
//
// An id with no stored face is [store.ErrNotFound], as is an `ID@face` whose
// face does not exist. families (readServices.Families) orders the faces in
// the message, as the resolver orders a Family (G18).
func readAddress(
	ctx context.Context, st addressLoader, families, world store.WorldScope, addr string,
) (*entity.Entity, error) {
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	typ, faces, err := storedFamily(ctx, st, parsed.ID())
	if err != nil {
		return nil, err
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("%w: %s", store.ErrNotFound, addr)
	}
	res, err := visibility.NewAllowAllResolver(st,
		visibility.WithFamilies(func() store.WorldScope { return families }))
	if err != nil { // coverage-ignore: defensive: NewAllowAllResolver only fails on a nil loader
		return nil, err
	}
	got, ok, err := res.Address(ctx, visibility.WorldOf(world), typ, parsed.String())
	if err != nil { // coverage-ignore: defensive: the allow-all gate never fails
		return nil, err
	}
	if ok {
		return got.Entity, nil
	}
	if _, named := parsed.Named(); named || slices.Contains(faces, entity.ImplicitFace) {
		return nil, fmt.Errorf("%w: %s", store.ErrNotFound, addr)
	}
	// Faces in chain order, then any the chain does not name by token, as
	// the resolver lists a Family. faces holds no implicit face here: that
	// case returned above.
	chain, _ := families.For(typ)
	rank := func(f entity.Face) int {
		if i := slices.Index(chain.Chain, f); i >= 0 {
			return i
		}
		return len(chain.Chain)
	}
	slices.SortStableFunc(faces, func(a, b entity.Face) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a.String(), b.String())
	})
	named := make([]string, len(faces))
	for i, face := range faces {
		named[i] = entity.FormatStateRef(parsed.ID(), face)
	}
	return nil, fmt.Errorf("%w: %s has faces; name one: %s", errFaceRequired, parsed.ID(), strings.Join(named, ", "))
}

// writeTarget resolves addr to the one face a face-level write edits, as
// the operator, through [visibility.Resolver.WriteTarget] in world (the
// default world). `ID@face` names that face; a bare id names the implicit
// face of a faceless type, and on a faced type is [errFaceRequired] naming
// its faces.
// An absent entity or face is [store.ErrNotFound].
func writeTarget(
	ctx context.Context, st addressLoader, families, world store.WorldScope, addr string,
) (entity.Ref, error) {
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return entity.Ref{}, fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	typ, _, err := storedFamily(ctx, st, parsed.ID())
	if err != nil {
		return entity.Ref{}, err
	}
	res, err := visibility.NewAllowAllResolver(st,
		visibility.WithFamilies(func() store.WorldScope { return families }))
	if err != nil { // coverage-ignore: defensive: NewAllowAllResolver only fails on a nil loader
		return entity.Ref{}, err
	}
	ref, ok, err := res.WriteTarget(ctx, visibility.WorldOf(world), typ, parsed)
	var amb *visibility.AmbiguousAddressError
	switch {
	case errors.As(err, &amb):
		return entity.Ref{}, fmt.Errorf("%w: %s", errFaceRequired, amb.Error())
	case err != nil: // coverage-ignore: defensive: the allow-all gate never fails
		return entity.Ref{}, err
	case !ok:
		return entity.Ref{}, fmt.Errorf("%w: %s", store.ErrNotFound, addr)
	}
	return ref, nil
}

// relationTail is the tail face an edge of relType from addr hangs on. A
// content-scoped edge belongs to one face, resolved as [writeTarget] does;
// an identity-scoped edge hangs on the entity, at the implicit face.
func relationTail(ctx context.Context, svc *readServices, addr, relType string) (entity.Ref, error) {
	if metamodel.IsContentScoped(svc.Meta, relType) {
		return writeTarget(ctx, svc.Store, svc.Families, svc.World, addr)
	}
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return entity.Ref{}, fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	return entity.Ref{ID: parsed.ID(), Face: entity.ImplicitFace}, nil
}

// storedFamily returns the type of id and every face it has a live row at,
// sorted, from one content-free header read. No face means no such entity.
func storedFamily(ctx context.Context, st store.EntityLister, id string) (string, []entity.Face, error) {
	headers, err := store.FamilyHeaders(ctx, st, id)
	if err != nil {
		return "", nil, fmt.Errorf("read the faces of %q: %w", id, err)
	}
	var (
		typ   string
		faces []entity.Face
	)
	for _, h := range headers {
		typ = h.Type
		faces = append(faces, h.Face)
	}
	slices.Sort(faces)
	return typ, faces, nil
}

// rowsInWorld loads, content-free, the rows world selects for ids in one
// query and returns them by id. The CLI uses it for neighbor titles, which
// come from the face the world selects (TKT-KQXVF7 design section 5.3). A
// neighbor the world resolves to no face is absent, and the caller shows its
// id. A failed read is logged and degrades the same way.
func rowsInWorld(
	ctx context.Context, st store.EntityLister, world store.WorldScope, ids []string,
) map[string]*entity.Entity {
	out := make(map[string]*entity.Entity, len(ids))
	if len(ids) == 0 {
		return out
	}
	for h, err := range store.ListEntityHeaders(ctx, st, store.EntityQuery{IDs: ids, Faces: store.InWorld(world)}) {
		if err != nil { // coverage-ignore: defensive: a failed title read degrades to showing ids
			slog.DebugContext(ctx, "neighbor titles: read failed", "ids", len(ids), "err", err)
			break
		}
		out[h.ID] = &entity.Entity{ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties}
	}
	return out
}

// requireAddressExists checks that an address names a stored entity, and, for
// `ID@face`, that face. A bare id of a faced entity exists: the checks that
// use this ask about the entity, not about a row. It answers the no-policy
// branch of the `acl` commands, which the aclmap engine answers the same way
// under a policy.
func requireAddressExists(ctx context.Context, st store.EntityLister, addr string) error {
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	_, faces, err := storedFamily(ctx, st, parsed.ID())
	if err != nil {
		return err
	}
	ref, named := parsed.Named()
	if len(faces) == 0 || (named && !slices.Contains(faces, ref.Face)) {
		return fmt.Errorf("entity %q not found", addr)
	}
	return nil
}
