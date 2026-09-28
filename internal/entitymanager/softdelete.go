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
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return SoftDeleted{}, false, err
	}
	for _, me := range marked {
		if me.ID == id && len(me.Entities) > 0 {
			return SoftDeleted{ID: me.ID, Type: me.Entities[0].Type, DeletedBy: me.DeletedBy}, true, nil
		}
	}
	return SoftDeleted{}, false, nil
}

// RestoreEntity undoes [SoftDeleteEntity]: the family and the relations that
// were hidden with it come back unchanged. Returns [ErrEntityNotFound] when id
// is not soft-deleted, including after it has been purged.
//
// It is authorized as a delete of the entity. A restore puts back exactly what
// the delete removed, so the grant that allowed the removal is the one that
// allows the undo. The check runs under [store.WithRevealed], because a local
// role is resolved from the entity's own relations and those are hidden while
// it is marked; without the reveal the restore would be judged without the
// grants the delete was judged with.
//
// The caller is responsible for the read gate: whether this principal may
// learn that id exists at all. See the data-entry restore handler.
func RestoreEntity(ctx context.Context, m *Manager, id string) (*entity.DeleteResult, error) {
	sd, ok := m.deps.Store.(store.SoftDeleteProvider)
	if !ok {
		return nil, ErrSoftDeleteUnsupported
	}
	found, ok, err := FindSoftDeleted(ctx, m, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	if aclErr := m.authorizeAndAudit(store.WithRevealed(ctx, id), acl.WriteRequest{
		Op:      acl.OpDelete,
		Subject: acl.NewEntitySubject(found.Type, id, ""),
	}); aclErr != nil {
		return nil, aclErr
	}

	res, err := sd.SoftDelete().Unmark(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		// Purged between the lookup and the unmark.
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	if err != nil {
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return nil, mapped
		}
		return nil, err
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
