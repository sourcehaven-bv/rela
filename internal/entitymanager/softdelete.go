package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
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
	// The principal needs the delete grant on every face, as DeleteEntity
	// requires: the mark removes them all.
	family, err := familyRows(ctx, m.deps.Store, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	authorized := make(familyAuthorization, len(family))
	if aclErr := m.authorizeFamily(ctx, acl.OpDelete, id, family, authorized); aclErr != nil {
		return nil, aclErr
	}

	// Collect, authorize and mark under one serialization, for the reason
	// DeleteEntity gives: an edge added between an outside check and the mark
	// would go without authorization.
	var marked familyChange
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		txCtx := store.ContextInTx(ctx)
		return softDeleteInTx(txCtx, m, tx, id, authorized, &marked)
	})
	if txErr != nil {
		// fs and mem cannot roll back, so a failure on a later mark leaves
		// the earlier ones in place. Record those, as DeleteEntity records a
		// partial cascade (issue #929); a mark that was rolled back left its
		// faces live and is recorded as nothing.
		marked.keep(func(r *store.DeleteResult) bool {
			return !facesStillStored(ctx, m.deps.Store, r.DeletedEntities)
		})
		recordSoftDelete(ctx, m, id, family[0], marked)
		return nil, txErr
	}
	return recordSoftDelete(ctx, m, id, family[0], marked), nil
}

// familyChange is what one soft delete or restore changed: the entity named
// in the call, and the owned entities that went or came back with it. Either
// part may be missing when the operation failed partway.
type familyChange struct {
	main  *store.DeleteResult
	owned []*store.DeleteResult
}

// keep drops every result that ok rejects.
func (c *familyChange) keep(ok func(*store.DeleteResult) bool) {
	if c.main != nil && !ok(c.main) {
		c.main = nil
	}
	kept := c.owned[:0]
	for _, r := range c.owned {
		if ok(r) {
			kept = append(kept, r)
		}
	}
	c.owned = kept
}

// softDeleteInTx is SoftDeleteEntity's critical section. It fills done as it
// marks, so a failure partway still reports what was marked.
func softDeleteInTx(
	ctx context.Context, m *Manager, tx store.Store, id string, authorized familyAuthorization, done *familyChange,
) error {
	// The family is re-read and re-authorized inside the serialization, for
	// the reason DeleteEntity gives.
	inTx, err := familyRows(ctx, tx, id)
	if err != nil {
		return err
	}
	if len(inTx) == 0 {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	if aErr := m.authorizeFamily(ctx, acl.OpDelete, id, inTx, authorized); aErr != nil {
		return aErr
	}
	incoming, err := collectIncidentRelations(ctx, tx, id, store.DirectionIncoming)
	if err != nil {
		return fmt.Errorf("collect incoming relations for %q: %w", id, err)
	}
	outgoing, err := collectIncidentRelations(ctx, tx, id, store.DirectionOutgoing)
	if err != nil {
		return fmt.Errorf("collect outgoing relations for %q: %w", id, err)
	}
	if aErr := m.authorizeCascadeRelations(ctx, tx, id, nil, incoming, outgoing); aErr != nil {
		return aErr
	}
	owned, err := prepareOwnedDeletes(ctx, m, tx, id, outgoing)
	if err != nil {
		return err
	}
	sd, ok := tx.(store.SoftDeleteProvider)
	if !ok { // unreachable: every in-tree Tx view carries the capability of its store
		return ErrSoftDeleteUnsupported
	}
	// The owner is marked FIRST, so the owning edges are hidden with it.
	// That is how RestoreEntity finds the owned entities again: from the
	// owner's hidden edges, see markedWithOwner.
	if done.main, err = sd.SoftDelete().MarkDeleted(ctx, id, deleterOf(ctx)); err != nil {
		return err
	}
	for _, o := range owned {
		r, mErr := sd.SoftDelete().MarkDeleted(ctx, o.id, deleterOf(ctx))
		if mErr != nil {
			return ownedDeleteError(id, mErr)
		}
		done.owned = append(done.owned, r)
	}
	return nil
}

// recordSoftDelete writes the audit records for what a soft delete marked
// and returns the result the caller reports. current is the face read before
// the transaction, which the entity's own record names.
func recordSoftDelete(
	ctx context.Context, m *Manager, id string, current *entity.Entity, marked familyChange,
) *entity.DeleteResult {
	out := &entity.DeleteResult{}
	if res := marked.main; res != nil {
		cascadeCtx := ctx
		if len(res.DeletedRelations) > 0 {
			cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
		}
		for _, rel := range res.DeletedRelations {
			m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted (undoable)")
		}
		m.recordEntityAudit(ctx, audit.OpDeleteEntity, current, "deleted (undoable)")
		out.DeletedEntities = append(out.DeletedEntities, current)
		out.DeletedRelations = append(out.DeletedRelations, res.DeletedRelations...)
	}
	ownedCtx := audit.WithTriggeredBy(ctx, ownerDeleteTrigger(id))
	for _, r := range marked.owned {
		for _, rel := range r.DeletedRelations {
			m.recordRelationAudit(ownedCtx, audit.OpDeleteRelation, rel, "deleted (undoable)")
		}
		if len(r.DeletedEntities) > 0 {
			m.recordEntityAudit(ownedCtx, audit.OpDeleteEntity, r.DeletedEntities[0], "deleted (undoable)")
			out.DeletedEntities = append(out.DeletedEntities, r.DeletedEntities[0])
		}
		out.DeletedRelations = append(out.DeletedRelations, r.DeletedRelations...)
	}
	return out
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
// The owned entities the delete took along come back with it, under the same
// checks (TKT-QO14GB). A restore that would bring back an owning edge the live
// graph no longer admits is refused with [ErrOwningRule].
//
// The caller is responsible for the read gate: whether this principal may
// learn that id exists at all. See the data-entry restore handler.
func RestoreEntity(ctx context.Context, m *Manager, id string) (*entity.DeleteResult, error) {
	if !SupportsSoftDelete(m) {
		return nil, ErrSoftDeleteUnsupported
	}
	var restored familyChange
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		txCtx := store.ContextInTx(ctx)
		return restoreInTx(txCtx, m, tx, id, &restored)
	})
	if txErr != nil {
		// As in SoftDeleteEntity: fs and mem keep an unmark that a later
		// failure did not undo, so record what is really back. A unique
		// conflict needs no pre-check for this: a mark holds its unique
		// values, so only a backend that rolls back reports one.
		restored.keep(func(r *store.DeleteResult) bool {
			return facesStored(ctx, m.deps.Store, r.DeletedEntities)
		})
		recordRestore(ctx, m, id, restored)
		if ok, mapped := mapUniquePropertyConflict(txErr); ok {
			return nil, mapped
		}
		return nil, txErr
	}
	return recordRestore(ctx, m, id, restored), nil
}

// restoreInTx is RestoreEntity's critical section. It fills done as it
// unmarks, so a failure partway still reports what came back.
func restoreInTx(ctx context.Context, m *Manager, tx store.Store, id string, done *familyChange) error {
	sd, ok := tx.(store.SoftDeleteProvider)
	if !ok { // unreachable: every in-tree Tx view carries the capability of its store
		return ErrSoftDeleteUnsupported
	}
	all, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return err
	}
	marked := make(map[string]store.MarkedEntity, len(all))
	for _, me := range all {
		marked[me.ID] = me
	}
	main, ok := marked[id]
	if !ok || len(main.Entities) == 0 {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	owned, err := markedWithOwner(ctx, m, tx, main, marked)
	if err != nil {
		return err
	}

	// Every family in the restore is revealed for every check. An owned
	// entity's local roles may come through its owner (inherit_roles_through
	// along an edge the owner's mark holds), and its edge to the owner may be
	// held by either mark.
	ids := []string{id}
	faces := slices.Clone(main.Entities)
	for _, o := range owned {
		ids = append(ids, o.ID)
		faces = append(faces, o.Entities...)
	}
	revealed := store.WithRevealed(ctx, ids...)
	src := markedSource{Store: tx, family: faces}
	if authErr := authorizeRestore(revealed, m, src, main); authErr != nil {
		return authErr
	}
	for _, o := range owned {
		if authErr := authorizeRestore(revealed, m, src, o); authErr != nil {
			return ownedRestoreError(id, authErr)
		}
	}
	for _, o := range owned {
		delete(marked, o.ID)
	}
	delete(marked, id)
	if oErr := checkRestoredOwning(ctx, m, tx, ids, marked); oErr != nil {
		return oErr
	}

	// Owned entities first: the owning edges are hidden with the owner, and
	// Unmark brings an edge back only when its other end is live.
	for _, o := range owned {
		r, uErr := sd.SoftDelete().Unmark(ctx, o.ID)
		if uErr != nil {
			return ownedRestoreError(id, uErr)
		}
		done.owned = append(done.owned, r)
	}
	res, err := sd.SoftDelete().Unmark(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	if err != nil {
		return err
	}
	done.main = res
	return nil
}

// recordRestore writes the audit records for what a restore brought back and
// returns the result the caller reports.
func recordRestore(ctx context.Context, m *Manager, id string, restored familyChange) *entity.DeleteResult {
	restoreCtx := audit.WithTriggeredBy(ctx, "restore-entity:"+id)
	out := &entity.DeleteResult{}
	if res := restored.main; res != nil {
		out.DeletedEntities = append(out.DeletedEntities, res.DeletedEntities...)
		out.DeletedRelations = append(out.DeletedRelations, res.DeletedRelations...)
	}
	for _, r := range restored.owned {
		for _, rel := range r.DeletedRelations {
			m.recordRelationAudit(restoreCtx, audit.OpCreateRelation, rel, "restored")
		}
		for _, e := range r.DeletedEntities {
			m.recordEntityAudit(restoreCtx, audit.OpRestoreEntity, e, "restored")
		}
		out.DeletedEntities = append(out.DeletedEntities, r.DeletedEntities...)
		out.DeletedRelations = append(out.DeletedRelations, r.DeletedRelations...)
	}
	if res := restored.main; res != nil {
		for _, rel := range res.DeletedRelations {
			m.recordRelationAudit(restoreCtx, audit.OpCreateRelation, rel, "restored")
		}
		for _, e := range res.DeletedEntities {
			m.recordEntityAudit(ctx, audit.OpRestoreEntity, e, "restored")
		}
	}
	return out
}

// facesStored reports whether every face is live in st. A read error counts
// as not live, so a restore that cannot be confirmed is not recorded.
func facesStored(ctx context.Context, st store.Store, faces []*entity.Entity) bool {
	for _, e := range faces {
		if _, err := st.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face}); err != nil {
			return false
		}
	}
	return len(faces) > 0
}

// authorizeRestore runs the delete checks for a restore of marked. ctx must
// reveal marked.ID, and src must answer for its faces.
func authorizeRestore(ctx context.Context, m *Manager, src markedSource, marked store.MarkedEntity) error {
	// Every face comes back, so every face needs the grant, as for the
	// delete.
	if err := m.authorizeFamily(ctx, acl.OpDelete, marked.ID, marked.Entities,
		make(familyAuthorization, len(marked.Entities))); err != nil {
		return err
	}
	incoming, err := collectIncidentRelations(ctx, src, marked.ID, store.DirectionIncoming)
	if err != nil {
		return fmt.Errorf("collect incoming relations for %q: %w", marked.ID, err)
	}
	outgoing, err := collectIncidentRelations(ctx, src, marked.ID, store.DirectionOutgoing)
	if err != nil {
		return fmt.Errorf("collect outgoing relations for %q: %w", marked.ID, err)
	}
	if err := m.authorizeCascadeRelations(ctx, src, marked.ID, marked.Entities, incoming, outgoing); err != nil {
		return fmt.Errorf("cannot restore %s: %w", marked.ID, err)
	}
	return nil
}

// markedSource answers for the faces of the marked entities a restore brings
// back, and passes every other call through. The cascade check resolves each
// edge's source type from the family's headers (lookupFamily), and a marked
// face is not readable, so an edge from one would resolve to no type and be
// refused.
//
// It embeds the [store.Store] interface, so a backend's [store.HeaderReader]
// is not visible through it and [store.ListEntityHeaders] falls back to
// ListEntities, which is the method that adds the marked faces.
type markedSource struct {
	store.Store
	family []*entity.Entity
}

func (s markedSource) GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	for _, e := range s.family {
		if e.ID == ref.ID && e.Face == ref.Face {
			return e, nil
		}
	}
	return s.Store.GetEntity(ctx, ref)
}

// ListEntities adds the marked faces whose id the query names to the live
// rows. Only an id-scoped query gets them; it is what a family lookup sends.
// A marked face is never also live, so nothing is listed twice.
func (s markedSource) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	live := s.Store.ListEntities(ctx, q)
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range live {
			if !yield(e, err) || err != nil {
				return
			}
		}
		for _, e := range s.family {
			if slices.Contains(q.IDs, e.ID) && !yield(e, nil) {
				return
			}
		}
	}
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
	// One delete version per face, as DeleteEntity records.
	head := res.DeletedEntities[0]
	for _, e := range res.DeletedEntities {
		m.recordEntityVersion(ctx, store.VersionOpDelete, e, "")
	}
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
