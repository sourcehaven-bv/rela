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

// LinkedCreate is a create of one entity, an edge between it and an
// existing entity, and a write of the caller's, that [CreateLinked] commits
// in one store transaction.
type LinkedCreate struct {
	Entity  *entity.Entity
	Options entity.CreateOptions
	// Authored holds the properties the caller wrote, which pass the field
	// write gate. A value the operator's configuration supplies, such as a
	// recipe's literal, is not the caller's and is not gated, as an
	// automation's value is not.
	//
	// Nil: accepted — every property in Entity.Properties is the caller's.
	Authored map[string]any
	// RelateType, when set, is the type of an edge between the new entity
	// and an existing one: from the new entity to RelateTo, or from
	// RelateFrom to the new entity. Exactly one of the two is set. A
	// content-scoped edge leaves the face the entity is created at, or
	// RelateFromFace.
	RelateType     string
	RelateTo       string
	RelateFrom     string
	RelateFromFace entity.Face
	// RelateBestEffort keeps the entity when the edge fails, and reports
	// the failure in [LinkedResult.RelateErr]. Set it for a store whose
	// transaction cannot roll back, where failing would not undo the entity
	// anyway.
	RelateBestEffort bool
	// BeforeWrite, when set, runs in the transaction before anything is
	// written, on the transaction's view. An error writes nothing.
	BeforeWrite func(ctx context.Context, view store.Store) error
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
// gate, the relation ACL, the edge's existing end and the relation-type
// tuple. In the transaction, still before the entity write, it checks the
// existing end's cardinality bound and ownership rules, so a full bound
// writes nothing on a store that cannot roll back. The relation ACL runs
// before the new entity has an id. That is sound
// because a relation write is decided from the source type and the
// caller's global roles only, never from the source id (pinned by
// acl.TestRelationWrite_DecisionIgnoresSourceID).
//
// The entity audit record, the alias notification, the automations and
// the cascade run after the commit, as they do for CreateEntity, followed
// by the relation audit record. A failure there is
// [LinkedResult.FollowUpErr], not an error, because the writes are
// committed. When the transaction fails on a store that cannot roll back
// and the entity stays written, those steps still run for it, so no entity
// exists without its audit record, and the error is a *[KeptEntityError]. A
// failed check, and a refused entity write, are reported so that
// [WroteNothing] knows them.
//
// A package function rather than a Manager method: Manager is at its
// method cap.
func CreateLinked(ctx context.Context, m *Manager, lc LinkedCreate) (*LinkedResult, error) {
	ctx = withStoreAttribution(ctx)
	t, err := prepareLinkedCreate(ctx, m, lc)
	if err != nil {
		return nil, beforeWriteError{err}
	}
	opts := lc.Options
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		return t.run(store.ContextInTx(ctx), view)
	})
	if err != nil {
		if t.created != nil && finishKept(ctx, m, t.created, t.warnings, t.written, opts.SkipAutomation) {
			return nil, &KeptEntityError{Entity: t.created, Err: err}
		}
		return nil, err
	}
	out := t.out
	out.Create, out.FollowUpErr = finishCreate(ctx, m, t.created, t.warnings, opts.SkipAutomation)
	if t.written != nil {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, t.written, "created")
		out.Relation = t.written
	}
	return out, nil
}

// linkedCreate is one [CreateLinked] call: what it prepared before the
// transaction, and what the transaction wrote.
type linkedCreate struct {
	m  *Manager
	lc LinkedCreate
	// key and rel are the edge, with the new entity's end blank until the
	// entity is written. Nil rel: no edge.
	key entity.RelationKey
	rel *entity.Relation

	out      *LinkedResult
	created  *entity.Entity
	warnings []entity.Warning
	written  *entity.Relation
}

// prepareLinkedCreate runs the checks that need no transaction: the
// entity's create checks, and the edge's when there is one.
func prepareLinkedCreate(ctx context.Context, m *Manager, lc LinkedCreate) (*linkedCreate, error) {
	e := lc.Entity
	if e == nil {
		return nil, errors.New("entitymanager: CreateLinked: entity is nil")
	}
	authored := lc.Authored
	if authored == nil {
		authored = e.Properties
	}
	if err := prepareCreate(ctx, m, e, lc.Options, authored); err != nil {
		return nil, err
	}
	t := &linkedCreate{m: m, lc: lc, out: &LinkedResult{}}
	if lc.RelateType != "" {
		var err error
		if t.key, t.rel, err = prepareLinkedRelation(ctx, m, lc); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// run is the transaction: BeforeWrite, the edge's checks, the entity, the
// edge, then InTx. txCtx is marked with [store.ContextInTx].
func (t *linkedCreate) run(txCtx context.Context, view store.Store) error {
	if t.lc.BeforeWrite != nil {
		if err := t.lc.BeforeWrite(txCtx, view); err != nil {
			return beforeWriteError{err}
		}
	}
	relate, err := t.checkRelation(txCtx, view)
	if err != nil {
		return err
	}
	deps := t.m.deps
	deps.Store = view
	c, w, err := createCore(txCtx, deps, t.lc.Entity.Type, createCoreArgs(t.lc.Entity, t.lc.Options))
	if err != nil {
		if isEntityStoreWriteError(err) {
			return err
		}
		// A refusal: createCore's only write is the entity row, and it was
		// refused.
		return beforeWriteError{err}
	}
	t.created, t.warnings = c, w
	if relate {
		if err := t.writeRelation(txCtx, view, c.ID); err != nil {
			return err
		}
	}
	if t.lc.InTx != nil {
		return t.lc.InTx(txCtx, view, c)
	}
	return nil
}

// checkRelation runs the edge's checks before the entity write, and
// reports whether the edge is still to be written. A best-effort edge that
// fails them is dropped and recorded on the result.
func (t *linkedCreate) checkRelation(ctx context.Context, view store.Store) (bool, error) {
	if t.rel == nil {
		return false, nil
	}
	err := checkLinkedRelation(ctx, view, t.m, t.key)
	switch {
	case err == nil:
		return true, nil
	case t.lc.RelateBestEffort:
		t.out.RelateErr = err
		return false, nil
	default:
		return false, beforeWriteError{err}
	}
}

// writeRelation fills in the new entity's end of the edge and writes it.
func (t *linkedCreate) writeRelation(ctx context.Context, view store.Store, id string) error {
	if t.lc.RelateFrom != "" {
		t.key.To, t.rel.To = id, id
	} else {
		t.key.From, t.rel.From = id, id
	}
	if err := writeLinkedRelation(ctx, view, t.m, t.key, t.rel); err != nil {
		if !t.lc.RelateBestEffort {
			return err
		}
		t.out.RelateErr = err
		return nil
	}
	t.written = t.rel
	return nil
}

// finishKept runs the post-commit steps of [CreateLinked] for an entity
// that a failed transaction left written, on a store that cannot roll
// back, and reports whether it stayed. On a store that rolled back the
// entity is gone and nothing runs. Their failures are logged: the caller
// already gets the transaction's error.
func finishKept(
	ctx context.Context, m *Manager, created *entity.Entity, warnings []entity.Warning,
	rel *entity.Relation, skipAutomation bool,
) bool {
	if _, err := m.deps.Store.GetEntity(ctx, created.Ref()); err != nil {
		return false
	}
	if _, err := finishCreate(ctx, m, created, warnings, skipAutomation); err != nil {
		slog.WarnContext(ctx, "entitymanager: CreateLinked: a step after the kept create failed",
			"entity", created.ID, "error", err)
	}
	if rel == nil {
		return true
	}
	if _, err := m.deps.Store.GetRelation(ctx, rel.Identity()); err == nil {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	}
	return true
}

// KeptEntityError is a [CreateLinked] failure after which the entity stays
// written: the transaction failed after the entity write, on a store that
// cannot roll back. The entity's audit record and automations ran.
type KeptEntityError struct {
	// Entity is the entity that stayed. Nil: never.
	Entity *entity.Entity
	Err    error
}

func (e *KeptEntityError) Error() string {
	return fmt.Sprintf("%s was created, but the rest of the create failed: %v", e.Entity.ID, e.Err)
}

func (e *KeptEntityError) Unwrap() error { return e.Err }

// prepareLinkedRelation runs the relation checks of lc before the new
// entity has an id. It returns the edge's key and relation with the new
// end still blank. The checks read only the new end's type and tail.
func prepareLinkedRelation(
	ctx context.Context, m *Manager, lc LinkedCreate,
) (entity.RelationKey, *entity.Relation, error) {
	e, opts := lc.Entity, lc.Options
	created := entityFamily{id: opts.ID, typ: e.Type, faces: []entity.Face{opts.Face}}
	switch {
	case lc.RelateTo != "" && lc.RelateFrom == "":
		key := entity.RelationKey{
			From: opts.ID, FromFace: relationTail(m, lc.RelateType, opts.Face), Type: lc.RelateType, To: lc.RelateTo,
		}
		rel, err := prepareRelationFrom(ctx, m, key, entity.RelationOptions{}, created, nil, nil)
		return key, rel, err
	case lc.RelateFrom != "" && lc.RelateTo == "":
		key := entity.RelationKey{
			From: lc.RelateFrom, FromFace: relationTail(m, lc.RelateType, lc.RelateFromFace),
			Type: lc.RelateType, To: opts.ID,
		}
		source, srcErr := lookupSource(ctx, m, key.From, key.FromFace)
		rel, err := prepareRelationFrom(ctx, m, key, entity.RelationOptions{}, source, srcErr, &created)
		return key, rel, err
	default:
		return entity.RelationKey{}, nil, errors.New(
			"entitymanager: CreateLinked: set exactly one of RelateTo and RelateFrom")
	}
}

// checkLinkedRelation checks the cardinality bound and the ownership rules
// of the edge key names, before the new entity is written. The new end is
// blank in key: it has no edges yet, so only the existing end is counted.
// The relation write checks again with both ids, as every relation create
// does.
func checkLinkedRelation(ctx context.Context, view store.Store, m *Manager, key entity.RelationKey) error {
	if err := checkRelationCapacity(ctx, view, m.deps.Meta, key, nil, nil); err != nil {
		return err
	}
	return checkOwningEdgeLeaving(ctx, m.deps.Meta, view, key, nil)
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
