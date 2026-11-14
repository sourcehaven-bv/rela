package entitymanager

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PatchWithRelations is [UpdateGrant.PatchEntity] together with a
// [Manager.ReplaceRelations] of edges at the patched entity, written in the
// store transaction of the entity update (TKT-EBARYBI).
//
// It is how a data-entry PATCH that changes properties and the edges a
// write-enforced relation rule reads is written. The update's relation rules
// are judged on the entity's new state and the edges the replace leaves, so
// setting a status and linking the plan it needs is accepted, and unlinking
// the last plan beside a property change is refused. Writing the two in one
// transaction is what keeps that verdict true: the update cannot be stored
// while the edges it counted on are refused.
//
// The replace is authorized and validated first, as ReplaceRelations does it,
// before the patch reads the entity. Inside the transaction every check of
// both writes runs before either writes, so on every backend a refusal stores
// nothing: the update's checks, then the replace's checks (existing edges,
// bounds, and the relation rules of the far ends), then the entity, then the
// edges. A store fault part way rolls back on Postgres and SQLite; on the
// file and memory stores it can leave the entity written without its edges,
// as any multi-row write there can.
//
// An error from the replace is a [*JoinedRelationError], so the caller can
// report it as a relation error. The created edges are returned in the order
// of creates. Their versions and audit records are written after the commit,
// also when the update's cascade fails after it.
//
// A function, not a method, to keep Manager under its plimsoll line.
func PatchWithRelations(
	ctx context.Context, g UpdateGrant, id string, p entity.Patch,
	creates []RelationCreate, removes []entity.RelationKey,
) (*entity.UpdateResult, []*entity.Relation, error) {
	m := g.m
	if m == nil {
		return nil, nil, errors.New("entitymanager: PatchWithRelations: grant not made by AuthorizeUpdate")
	}
	ctx = withStoreAttribution(ctx)
	removeKeys, leaving, err := uniqueRemoves(removes)
	if err != nil {
		return nil, nil, &JoinedRelationError{Err: err}
	}
	rp := &relationReplace{m: m, creates: creates, removes: removeKeys, leaving: leaving}
	if prepErr := rp.prepare(ctx); prepErr != nil {
		return nil, nil, &JoinedRelationError{Err: prepErr}
	}
	ids, err := lineageIDs(ctx, m.deps.RelationVersionRecorder, rp.removes)
	if err != nil {
		return nil, nil, err
	}
	j := &joinedReplace{rp: rp, id: g.ref.ID}
	res, err := patchEntity(context.WithValue(ctx, joinedReplaceKey{}, j), m, id, p,
		patchPre{grant: &g, recheck: true})
	if j.committed {
		for _, r := range j.deleted {
			m.recordRelationVersion(ctx, store.VersionOpDelete, r, ids[r.Identity()], "", "", "")
			m.recordRelationAudit(ctx, audit.OpDeleteRelation, r, "deleted")
		}
		for _, rel := range rp.rels {
			m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
		}
	}
	if err != nil {
		return nil, nil, err
	}
	return res, rp.rels, nil
}

// JoinedRelationError is a refusal or failure of the replace in
// [PatchWithRelations]. Nothing was written.
type JoinedRelationError struct{ Err error }

func (e *JoinedRelationError) Error() string { return e.Err.Error() }
func (e *JoinedRelationError) Unwrap() error { return e.Err }

type joinedReplaceKey struct{}

// joinedReplace is the replace of one [PatchWithRelations] call, carried on
// the ctx to the update of entity id.
type joinedReplace struct {
	rp *relationReplace
	id string
	// deleted is what the last attempt's transaction deleted; committed is
	// set once that transaction committed. The update writes the replace
	// once: a later update of id in the same request, such as one its
	// cascade makes, does not.
	deleted   []*entity.Relation
	committed bool
}

// joinedFor returns the replace joined to the update of entity id, or nil.
func joinedFor(ctx context.Context, id string) *joinedReplace {
	j, ok := ctx.Value(joinedReplaceKey{}).(*joinedReplace)
	if !ok || j.committed || j.id != id {
		return nil
	}
	return j
}

// adds returns the keys of the edges the replace creates.
func (j *joinedReplace) adds() []entity.RelationKey {
	keys := make([]entity.RelationKey, len(j.rp.creates))
	for i, c := range j.rp.creates {
		keys[i] = c.Key
	}
	return keys
}

// check is the replace's part of the update's transaction checks. The rules
// of the patched entity are left to the update's own check, which judges
// them on its new state.
func (j *joinedReplace) check(ctx context.Context) error {
	view := store.TxView(ctx)
	if view == nil {
		// coverage-ignore: invariant: writeChecked puts the view on the ctx of every txCheck
		return errors.New("joined relation check: no transaction view")
	}
	j.deleted = nil
	if err := j.rp.check(ctx, view, j.id); err != nil {
		return &JoinedRelationError{Err: err}
	}
	return nil
}

// after returns write followed by the replace's writes, on the same view.
func (j *joinedReplace) after(ctx context.Context, write func(store.Store) error) func(store.Store) error {
	return func(view store.Store) error {
		if err := write(view); err != nil {
			return err
		}
		deleted, err := j.rp.apply(store.ContextInTx(ctx), view)
		if err != nil {
			return &JoinedRelationError{Err: err}
		}
		j.deleted = deleted
		return nil
	}
}
