package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Soft delete backs the data-entry Undo toast. A soft-deleted entity is
// deleted in every observable way: it is gone from reads, search and the
// event feed, and its incident relations go with it. The store keeps the rows
// for a short while (see [store.SoftDeleter]) so [RestoreEntity] can bring
// them back unchanged, and [PurgeSoftDeleted] removes them for real once the
// undo window has passed.
//
// These are package functions rather than Manager methods for the reason
// CopiesForSource gives: Manager sits on its plimsoll method load line.

// ErrSoftDeleteUnsupported is returned when the store has no
// [store.SoftDeleter]. Callers fall back to [Manager.DeleteEntity].
var ErrSoftDeleteUnsupported = errors.New("entitymanager: store does not support soft delete")

// SupportsSoftDelete reports whether m's store can soft-delete.
func SupportsSoftDelete(m *Manager) bool {
	_, ok := m.deps.Store.(store.SoftDeleteProvider)
	return ok
}

// SoftDeletes carries the soft-delete functions as methods, so a consumer can
// hold them behind its own narrow interface. Like [CopyAffordances] it only
// forwards; every method authorizes inside the function it calls.
type SoftDeletes struct{ M *Manager }

// SoftDeleteEntity forwards to [SoftDeleteEntity].
func (s SoftDeletes) SoftDeleteEntity(ctx context.Context, id string) (*entity.DeleteResult, error) {
	return SoftDeleteEntity(ctx, s.M, id)
}

// FindSoftDeleted forwards to [FindSoftDeleted].
func (s SoftDeletes) FindSoftDeleted(ctx context.Context, id string) (SoftDeleted, bool, error) {
	return FindSoftDeleted(ctx, s.M, id)
}

// RestoreEntity forwards to [RestoreEntity].
func (s SoftDeletes) RestoreEntity(ctx context.Context, id string) (*entity.DeleteResult, error) {
	return RestoreEntity(ctx, s.M, id)
}

// SoftDeleteEntity deletes the whole family of id and its incident relations
// so that [RestoreEntity] can undo it. Authorization and the cascade check are
// those of DeleteEntity with cascade set: the principal needs the delete grant
// on the entity and on every relation that goes with it.
//
// Nothing is written to version history here; the rows are unchanged, and a
// restore must not leave a delete marker behind. [PurgeSoftDeleted] records
// the delete versions when the rows really go.
func SoftDeleteEntity(ctx context.Context, m *Manager, id string) (*entity.DeleteResult, error) {
	if !SupportsSoftDelete(m) {
		return nil, ErrSoftDeleteUnsupported
	}
	// Fails closed on a non-not-found error, as DeleteEntity does.
	current, err := anyFaceOf(ctx, m.deps.Store, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpDelete,
		Subject: acl.NewEntitySubject(current.Type, id, current.Face),
	}); aclErr != nil {
		return nil, aclErr
	}

	// Collect, authorize and mark under one serialization, for the reason
	// DeleteEntity gives: an edge added between an outside check and the mark
	// would go without authorization.
	var res *store.DeleteResult
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		incoming, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionIncoming)
		if cErr != nil {
			return fmt.Errorf("collect incoming relations for %q: %w", id, cErr)
		}
		outgoing, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionOutgoing)
		if cErr != nil {
			return fmt.Errorf("collect outgoing relations for %q: %w", id, cErr)
		}
		if aErr := m.authorizeCascadeRelations(ctx, tx, id, incoming, outgoing); aErr != nil {
			return aErr
		}
		sd, ok := tx.(store.SoftDeleteProvider)
		if !ok { // unreachable: every in-tree Tx view carries the capability of its store
			return ErrSoftDeleteUnsupported
		}
		var mErr error
		res, mErr = sd.SoftDelete().MarkDeleted(ctx, id, deleterOf(ctx))
		return mErr
	})
	if txErr != nil {
		return nil, txErr
	}

	cascadeCtx := ctx
	if len(res.DeletedRelations) > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
	}
	for _, rel := range res.DeletedRelations {
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted (undoable)")
	}
	m.recordEntityAudit(ctx, audit.OpDeleteEntity, current, "deleted (undoable)")

	return &entity.DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: res.DeletedRelations,
	}, nil
}

// SoftDeleted describes one soft-deleted entity, for the restore read gate.
type SoftDeleted struct {
	ID        string
	Type      string
	DeletedBy string
}

// FindSoftDeleted returns the soft-deleted entity id, or ok=false when there
// is none (or the store cannot soft-delete).
func FindSoftDeleted(ctx context.Context, m *Manager, id string) (SoftDeleted, bool, error) {
	sd, ok := m.deps.Store.(store.SoftDeleteProvider)
	if !ok {
		return SoftDeleted{}, false, nil
	}
	me, ok, err := findMarked(ctx, sd, id)
	if !ok || err != nil {
		return SoftDeleted{}, false, err
	}
	return SoftDeleted{ID: me.ID, Type: me.Entities[0].Type, DeletedBy: me.DeletedBy}, true, nil
}

// findMarked returns the marked family of id, which holds at least one
// entity when ok is true. Its default face, when it has one, comes first.
func findMarked(ctx context.Context, sd store.SoftDeleteProvider, id string) (store.MarkedEntity, bool, error) {
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return store.MarkedEntity{}, false, err
	}
	for _, me := range marked {
		if me.ID == id && len(me.Entities) > 0 {
			return me, true, nil
		}
	}
	return store.MarkedEntity{}, false, nil
}

// RestoreEntity undoes [SoftDeleteEntity]: the family and the relations that
// were hidden with it come back unchanged. Returns [ErrEntityNotFound] when id
// is not soft-deleted, including after it has been purged.
//
// It needs exactly the grants the delete needed: the delete grant on the
// entity, and the delete grant on every relation that comes back with it (the
// cascade check). A restore puts back what the delete removed, so a principal
// who could not have removed an edge must not be able to put it back.
//
// The checks run under [store.WithRevealed], because the entity's relations
// are hidden while it is marked: a local role is resolved from them, and the
// cascade check has to see them. They run in the same transaction as the
// unmark, for the reason SoftDeleteEntity collects and marks in one.
//
// The caller is responsible for the read gate: whether this principal may
// learn that id exists at all. See the data-entry restore handler.
func RestoreEntity(ctx context.Context, m *Manager, id string) (*entity.DeleteResult, error) {
	if _, ok := m.deps.Store.(store.SoftDeleteProvider); !ok {
		return nil, ErrSoftDeleteUnsupported
	}
	var res *store.DeleteResult
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		sd, ok := tx.(store.SoftDeleteProvider)
		if !ok { // unreachable: every in-tree Tx view carries the capability of its store
			return ErrSoftDeleteUnsupported
		}
		marked, ok, err := findMarked(ctx, sd, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
		}
		if err := authorizeRestore(store.WithRevealed(ctx, id), m, tx, marked); err != nil {
			return err
		}
		res, err = sd.SoftDelete().Unmark(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
		}
		return err
	})
	if txErr != nil {
		if ok, mapped := mapUniquePropertyConflict(txErr); ok {
			return nil, mapped
		}
		return nil, txErr
	}

	restoreCtx := audit.WithTriggeredBy(ctx, "restore-entity:"+id)
	for _, rel := range res.DeletedRelations {
		m.recordRelationAudit(restoreCtx, audit.OpCreateRelation, rel, "restored")
	}
	for _, e := range res.DeletedEntities {
		m.recordEntityAudit(ctx, audit.OpRestoreEntity, e, "restored")
	}
	return &entity.DeleteResult{DeletedEntities: res.DeletedEntities, DeletedRelations: res.DeletedRelations}, nil
}

// authorizeRestore runs the delete checks for a restore of marked. ctx must
// reveal marked.ID.
func authorizeRestore(ctx context.Context, m *Manager, tx store.Store, marked store.MarkedEntity) error {
	head := marked.Entities[0]
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpDelete,
		Subject: acl.NewEntitySubject(head.Type, marked.ID, ""),
	}); err != nil {
		return err
	}
	incoming, err := collectIncidentRelations(ctx, tx, marked.ID, store.DirectionIncoming)
	if err != nil {
		return fmt.Errorf("collect incoming relations for %q: %w", marked.ID, err)
	}
	outgoing, err := collectIncidentRelations(ctx, tx, marked.ID, store.DirectionOutgoing)
	if err != nil {
		return fmt.Errorf("collect outgoing relations for %q: %w", marked.ID, err)
	}
	// The cascade check resolves each edge's source type with GetEntity. The
	// marked entity is not readable, so an outgoing edge would resolve to no
	// type and be refused; markedSource answers for it.
	src := markedSource{Store: tx, head: head}
	if err := m.authorizeCascadeRelations(ctx, src, marked.ID, incoming, outgoing); err != nil {
		return fmt.Errorf("cannot restore %s: %w", marked.ID, err)
	}
	return nil
}

// markedSource answers GetEntity for one marked entity and passes every other
// call through.
type markedSource struct {
	store.Store
	head *entity.Entity
}

func (s markedSource) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	if id == s.head.ID {
		return s.head, nil
	}
	return s.Store.GetEntity(ctx, id)
}

// PurgeSoftDeleted removes, for real, every soft-deleted entity marked before
// cutoff, and returns how many it purged. It is the garbage-collection job
// behind the undo window (see appbuild's soft-delete GC).
//
// Each purge is recorded as the deleter's action: the delete versions and the
// audit record carry the principal who deleted the entity, with the tool set
// to [principal.ToolSoftDeleteGC] so the log shows who finished it.
//
// A failure on one entity is returned after the others have been tried, so
// one stuck row does not hold back the rest.
func PurgeSoftDeleted(ctx context.Context, m *Manager, cutoff time.Time) (int, error) {
	sd, ok := m.deps.Store.(store.SoftDeleteProvider)
	if !ok {
		return 0, nil
	}
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return 0, err
	}
	purged := 0
	var errs []error
	for _, me := range marked {
		if !me.DeletedAt.Before(cutoff) {
			break // oldest first, so the rest are newer
		}
		res, pErr := sd.SoftDelete().PurgeMarked(ctx, me.ID)
		if errors.Is(pErr, store.ErrNotFound) {
			continue // restored or purged meanwhile
		}
		if pErr != nil {
			errs = append(errs, fmt.Errorf("purge %s: %w", me.ID, pErr))
			continue
		}
		purged++
		recordPurge(purgeContext(ctx, me.DeletedBy), m, me.ID, res)
	}
	return purged, errors.Join(errs...)
}

// recordPurge writes the delete versions, alias notice and audit record for
// one purged family.
func recordPurge(ctx context.Context, m *Manager, id string, res *store.DeleteResult) {
	if len(res.DeletedEntities) == 0 {
		return
	}
	// The default face leads; DeleteEntity likewise records one version.
	head := res.DeletedEntities[0]
	m.recordEntityVersion(ctx, store.VersionOpDelete, head, "")
	m.notifyAliasesOfDelete(ctx, id)
	cascadeTB := "cascade:delete-entity:" + id
	for _, rel := range res.DeletedRelations {
		m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", cascadeTB)
	}
	m.recordEntityAudit(ctx, audit.OpPurgeDeletedEntity, head,
		fmt.Sprintf("purged after soft delete (%d relations)", len(res.DeletedRelations)))
}

// purgeContext attributes a purge to the principal who deleted the entity.
func purgeContext(ctx context.Context, deletedBy string) context.Context {
	if deletedBy == "" {
		deletedBy = principal.Unknown
	}
	return principal.With(ctx, principal.Principal{User: deletedBy, Tool: principal.ToolSoftDeleteGC})
}

// deleterOf is the user recorded on a mark: the acting principal, or empty
// when the request carried none.
func deleterOf(ctx context.Context) string {
	if u := principal.From(ctx).User; u != principal.Unknown {
		return u
	}
	return ""
}
