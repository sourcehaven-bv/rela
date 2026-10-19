package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LinkedCreate is a create of one entity, an edge from it, and a write of
// the caller's, that [CreateLinked] commits in one store transaction.
type LinkedCreate struct {
	Entity  *entity.Entity
	Options entity.CreateOptions
	// RelateType, when set, is the type of an edge from the new entity to
	// the entity RelateTo. A content-scoped edge leaves the face the entity
	// is created at.
	RelateType string
	RelateTo   string
	// RelateBestEffort keeps the entity when the edge fails, and reports
	// the failure in [LinkedResult.RelateErr]. Set it for a store whose
	// transaction cannot roll back, where failing would not undo the entity
	// anyway.
	RelateBestEffort bool
	// InTx, when set, runs in the transaction after the entity and the
	// edge are written, on the transaction's view. An error rolls the
	// transaction back where the store can.
	InTx func(ctx context.Context, view store.Store, created *entity.Entity) error
}

// LinkedResult is the outcome of [CreateLinked].
type LinkedResult struct {
	Create   *entity.CreateResult
	Relation *entity.Relation
	// RelateErr is the edge failure [LinkedCreate.RelateBestEffort] kept.
	RelateErr error
	// FollowUpErr is a failure of the steps after the commit: the
	// automations and the cascade. The entity and the edge stay written.
	FollowUpErr error
}

// CreateLinked creates lc.Entity, the edge lc names and lc.InTx's write in
// one [store.Store.Tx]. On a store with a real transaction (postgres,
// sqlite) they commit together or not at all. On the filesystem store the
// transaction only excludes other writers: an error after the entity is
// written leaves it written.
//
// It checks what [Manager.CreateEntity] and [Manager.CreateRelation] check,
// as the caller, before anything is written: the create ACL, the field
// gate, the relation ACL, the edge's target and the relation-type tuple.
// The relation ACL runs before the new entity has an id. That is sound
// because a relation write is decided from the source type and the
// caller's global roles only, never from the source id (pinned by
// acl.TestRelationWrite_DecisionIgnoresSourceID).
//
// The entity audit record, the alias notification, the automations and
// the cascade run after the commit, as they do for CreateEntity, followed
// by the relation audit record. A failure there is
// [LinkedResult.FollowUpErr], not an error, because the writes are
// committed. When the transaction fails on a store that cannot roll back
// and the entity stays written, those steps still run for it before the
// error is returned, so no entity exists without its audit record. A
// failed check, and a refused entity write, are reported so that
// [WroteNothing] knows them.
//
// A package function rather than a Manager method: Manager is at its
// method cap.
func CreateLinked(ctx context.Context, m *Manager, lc LinkedCreate) (*LinkedResult, error) {
	ctx = withStoreAttribution(ctx)
	e, opts := lc.Entity, lc.Options
	if e == nil {
		return nil, errors.New("entitymanager: CreateLinked: entity is nil")
	}
	if err := prepareCreate(ctx, m, e, opts); err != nil {
		return nil, beforeWriteError{err}
	}
	var key entity.RelationKey
	var rel *entity.Relation
	if lc.RelateType != "" {
		// The new entity has no id yet. The relation checks do not read the
		// source, only its type and tail, so they run before the writes.
		key = entity.RelationKey{
			From: opts.ID, FromFace: relationTail(m, lc.RelateType, opts.Face), Type: lc.RelateType, To: lc.RelateTo,
		}
		source := entityFamily{id: opts.ID, typ: e.Type, faces: []entity.Face{opts.Face}}
		var err error
		rel, err = prepareRelationFrom(ctx, m, key, entity.RelationOptions{}, source, nil)
		if err != nil {
			return nil, beforeWriteError{err}
		}
	}

	out := &LinkedResult{}
	var created *entity.Entity
	var warnings []entity.Warning
	var written *entity.Relation
	err := m.deps.Store.Tx(ctx, func(view store.Store) error {
		txCtx := store.ContextInTx(ctx)
		deps := m.deps
		deps.Store = view
		c, w, err := createCore(txCtx, deps, e.Type, createCoreArgs(e, opts))
		if err != nil {
			if isEntityStoreWriteError(err) {
				return err
			}
			// A refusal: createCore's only write is the entity row, and
			// it was refused.
			return beforeWriteError{err}
		}
		created, warnings = c, w
		if rel != nil {
			key.From, rel.From = c.ID, c.ID
			if err := writeLinkedRelation(txCtx, view, m, key, rel); err != nil {
				if !lc.RelateBestEffort {
					return err
				}
				out.RelateErr = err
			} else {
				written = rel
			}
		}
		if lc.InTx != nil {
			return lc.InTx(txCtx, view, c)
		}
		return nil
	})
	if err != nil {
		if created != nil {
			finishKept(ctx, m, created, warnings, written, opts.SkipAutomation)
		}
		return nil, err
	}
	res, err := finishCreate(ctx, m, created, warnings, opts.SkipAutomation)
	out.Create, out.FollowUpErr = res, err
	if written != nil {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, written, "created")
		out.Relation = written
	}
	return out, nil
}

// finishKept runs the post-commit steps of [CreateLinked] for an entity
// that a failed transaction left written, on a store that cannot roll
// back. On a store that rolled back the entity is gone and nothing runs.
// Their failures are logged: the caller already gets the transaction's
// error.
func finishKept(
	ctx context.Context, m *Manager, created *entity.Entity, warnings []entity.Warning,
	rel *entity.Relation, skipAutomation bool,
) {
	if _, err := m.deps.Store.GetEntity(ctx, created.Ref()); err != nil {
		return
	}
	if _, err := finishCreate(ctx, m, created, warnings, skipAutomation); err != nil {
		slog.WarnContext(ctx, "entitymanager: CreateLinked: a step after the kept create failed",
			"entity", created.ID, "error", err)
	}
	if rel == nil {
		return
	}
	if _, err := m.deps.Store.GetRelation(ctx, rel.Identity()); err == nil {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	}
}

// CheckCreate answers the create ACL question [Manager.CreateEntity] asks
// for an entity of typ at face, as the caller on ctx, without writing or
// auditing: an affordance probe. It reads no store; a refusal is the
// *[acl.ForbiddenError] the create would return, and a face the type
// does not admit is the face error.
//
// A package function rather than a Manager method: Manager is at its
// method cap.
func CheckCreate(ctx context.Context, m *Manager, typ string, face entity.Face) error {
	if err := m.deps.requireCreateFaceFor(typ, face); err != nil {
		return err
	}
	return m.authorizeAndAudit(withAffordanceProbe(ctx), acl.WriteRequest{
		Op: acl.OpCreate, Subject: acl.NewEntitySubject(typ, "", face),
	})
}

// beforeWriteError is a [CreateLinked] failure after which nothing was
// written: a refused check before the transaction, or a refused entity
// write inside it; see [WroteNothing].
type beforeWriteError struct{ err error }

func (e beforeWriteError) Error() string { return e.err.Error() }
func (e beforeWriteError) Unwrap() error { return e.err }

// WroteNothing reports whether err is a [CreateLinked] failure after which
// nothing was written: a refused check of the entity or the edge, or a
// refused entity write such as a validation failure, an illegal
// state-machine entry or an existing id. An error it does not recognize
// may still have written nothing.
func WroteNothing(err error) bool {
	_, ok := errors.AsType[beforeWriteError](err)
	return ok
}

// isEntityStoreWriteError reports whether err is a [createCore] failure of
// the store write itself, rather than a refusal. Whether such a write left
// anything behind is unknown.
func isEntityStoreWriteError(err error) bool {
	_, ok := errors.AsType[entityStoreWriteError](err)
	return ok
}

// writeLinkedRelation writes rel on view, reporting a duplicate the way
// [Manager.CreateRelation] does.
func writeLinkedRelation(
	ctx context.Context, view store.Store, m *Manager, key entity.RelationKey, rel *entity.Relation,
) error {
	err := writeRelationCreate(ctx, view, m.deps.Meta, key, rel, nil)
	if errors.Is(err, store.ErrConflict) {
		return fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists,
			entity.FormatStateRef(key.From, key.FromFace), key.Type, key.To)
	}
	return err
}

// relationTail is the tail an edge of relType from an entity at face
// leaves: the face for a content-scoped relation, none for an identity
// one or an unknown type.
func relationTail(m *Manager, relType string, face entity.Face) entity.Face {
	def, ok := m.deps.Meta.GetRelationDef(relType)
	if !ok || def.Scope.IsIdentity() {
		return ""
	}
	return face
}
