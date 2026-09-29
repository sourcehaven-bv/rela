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
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// errFaceRequired is returned when a bare id names an entity that the world
// resolves to no face: a faced type in the default world until TKT-7IZHP0.
// The message names the entity's faces so the operator can pick one.
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
// `ID@face` reads that face. A bare id resolves in world, which is the
// default world from the compiled worlds (readServices.World). The default
// world reads the implicit face, which a faced type does not have
// (DEC-NPZICR), so a bare id of a faced entity fails with [errFaceRequired]
// and a message naming its faces, as `rela attach` does. When TKT-7IZHP0
// generates a default world, the same bare id resolves and the error goes
// away without a change here.
//
// An id with no stored face is [store.ErrNotFound], as is an `ID@face` whose
// face does not exist.
func readAddress(ctx context.Context, st addressLoader, world store.WorldScope, addr string) (*entity.Entity, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	typ, faces, err := storedFamily(ctx, st, ref.ID)
	if err != nil {
		return nil, err
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("%w: %s", store.ErrNotFound, addr)
	}
	res, err := visibility.NewAllowAllResolver(st)
	if err != nil { // coverage-ignore: defensive: NewAllowAllResolver only fails on a nil loader
		return nil, err
	}
	got, ok, err := res.Address(ctx, visibility.WorldOf(world), typ, ref.String())
	if err != nil { // coverage-ignore: defensive: the allow-all gate never fails
		return nil, err
	}
	if ok {
		return got.Entity, nil
	}
	if !ref.Face.IsDefault() || slices.Contains(faces, "") {
		return nil, fmt.Errorf("%w: %s", store.ErrNotFound, addr)
	}
	named := make([]string, len(faces))
	for i, face := range faces {
		named[i] = entity.FormatStateRef(ref.ID, face)
	}
	return nil, fmt.Errorf("%w: %s has faces; name one: %s", errFaceRequired, ref.ID, strings.Join(named, ", "))
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
// neighbor the world resolves to no face, such as a faced type in the default
// world until TKT-7IZHP0, is absent, and the caller shows its id. A failed
// read is logged and degrades the same way.
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
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return fmt.Errorf("invalid entity address %q: %w", addr, err)
	}
	_, faces, err := storedFamily(ctx, st, ref.ID)
	if err != nil {
		return err
	}
	if len(faces) == 0 || (!ref.Face.IsDefault() && !slices.Contains(faces, ref.Face)) {
		return fmt.Errorf("entity %q not found", addr)
	}
	return nil
}
