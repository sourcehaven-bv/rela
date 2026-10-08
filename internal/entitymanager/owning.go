package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrOwningRule is returned when a new edge of an `owning:` relation type
// would break ownership (TKT-QO14GB): an entity owning itself, an entity with
// a second owner, or a second level of ownership.
var ErrOwningRule = errors.New("owning rule")

// CheckOwningEdge refuses a new edge of an owning relation type that would
// break ownership. It is a no-op for any other relation type.
//
// Call it with the store handle of the transaction that writes the edge, so
// the edges it reads and the edge being written are serialized together.
// Every write path that creates relations must call it, including the ones
// that write the store directly: automation `create_relation`, copies and
// the data-entry type-allowlist fallback.
//
// An existing edge identical to key is not counted as a second owner. A
// re-create of that edge is then left to the store's create, which reports
// the conflict as such.
//
// Importer, data migrations and hand-edited files are operator-trusted and do
// not pass here, so readers must still tolerate irregular ownership.
func CheckOwningEdge(ctx context.Context, meta *metamodel.Metamodel, st store.Store, key entity.RelationKey) error {
	if !metamodel.IsOwning(meta, key.Type) {
		return nil
	}
	if key.From == key.To {
		return fmt.Errorf("%w: %s cannot own itself", ErrOwningRule, key.From)
	}
	identical := func(rel *entity.Relation) bool { return rel.From == key.From && rel.Type == key.Type }
	checks := []struct {
		id   string
		dir  store.Direction
		skip func(*entity.Relation) bool
	}{
		{key.To, store.DirectionIncoming, identical}, // To already has an owner
		{key.From, store.DirectionIncoming, nil},     // From is itself owned
		{key.To, store.DirectionOutgoing, nil},       // To owns something
	}
	for _, c := range checks {
		n, err := countOwningEdges(ctx, meta, st, c.id, c.dir, c.skip)
		if err != nil {
			return err
		}
		if n > 0 {
			return owningConflict(key)
		}
	}
	return nil
}

// owningConflict is the refusal for an edge that clashes with an existing
// owning edge. The clashing edge may lead to an entity the caller cannot read,
// so the message does not say which rule it broke: naming it would tell the
// caller whether the hidden edge is an owner of To, an owner of From, or a
// child of To.
func owningConflict(key entity.RelationKey) error {
	return fmt.Errorf("%w: %s --%s--> %s would break ownership: an entity has at most one owner, "+
		"and an owned entity owns nothing", ErrOwningRule, key.From, key.Type, key.To)
}

// countOwningEdges counts the owning edges on id in one direction, leaving
// out the ones skip reports. skip may be nil.
func countOwningEdges(
	ctx context.Context, meta *metamodel.Metamodel, st store.Store, id string, dir store.Direction,
	skip func(*entity.Relation) bool,
) (int, error) {
	n := 0
	for rel, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: dir}) {
		if err != nil {
			return 0, err
		}
		if metamodel.IsOwning(meta, rel.Type) && (skip == nil || !skip(rel)) {
			n++
		}
	}
	return n, nil
}

// ownedDeletion is one owned entity that its owner's cascade delete takes
// along. prepareOwnedDeletes fills id, authorized and capture before any
// write; deleteOwned fills res.
type ownedDeletion struct {
	id         string
	authorized familyAuthorization
	capture    *cascadeCapture
	res        *store.DeleteResult
}

// ownedChildIDs returns the targets of the owning edges in outgoing, once
// each and in edge order. A self edge is skipped: the owner is being deleted
// anyway, and hand-edited data may contain one.
func ownedChildIDs(meta *metamodel.Metamodel, owner string, outgoing []*entity.Relation) []string {
	var ids []string
	seen := map[string]bool{owner: true}
	for _, rel := range outgoing {
		if rel == nil || !metamodel.IsOwning(meta, rel.Type) || seen[rel.To] {
			continue
		}
		seen[rel.To] = true
		ids = append(ids, rel.To)
	}
	return ids
}

// The functions below take *Manager rather than being methods on it, for the
// reason softdelete.go gives: Manager sits on its plimsoll method load line.

// prepareOwnedDeletes authorizes the delete of every entity owner owns, with
// the same checks a direct cascade delete of each would run: the delete grant
// on every face, and on every relation that goes with it. It writes nothing,
// so a denial leaves the store untouched on every backend.
//
// The error never names an owned entity. Whether the owner's caller may read
// the entities it owns is not known here, and the HTTP 403 carries only the
// decision, which names types and roles.
func prepareOwnedDeletes(
	ctx context.Context, m *Manager, tx store.Store, owner string, outgoing []*entity.Relation,
) ([]ownedDeletion, error) {
	var owned []ownedDeletion
	for _, id := range ownedChildIDs(m.deps.Meta, owner, outgoing) {
		family, err := familyRows(ctx, tx, id)
		if err != nil {
			return nil, ownedDeleteError(owner, err)
		}
		if len(family) == 0 {
			continue // a dangling edge: the store removes it with the owner
		}
		authorized := make(familyAuthorization, len(family))
		if aErr := m.authorizeFamily(ctx, acl.OpDelete, id, family, authorized); aErr != nil {
			return nil, ownedDeleteError(owner, aErr)
		}
		incoming, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionIncoming)
		if cErr != nil {
			return nil, ownedDeleteError(owner, cErr)
		}
		out, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionOutgoing)
		if cErr != nil {
			return nil, ownedDeleteError(owner, cErr)
		}
		if aErr := m.authorizeCascadeRelations(ctx, tx, id, nil, incoming, out); aErr != nil {
			return nil, ownedDeleteError(owner, aErr)
		}
		owned = append(owned, ownedDeletion{
			id:         id,
			authorized: authorized,
			capture:    &cascadeCapture{incoming: incoming, outgoing: out},
		})
	}
	return owned, nil
}

// deleteOwned deletes the prepared owned entities through tx, each with its
// relations. It stops at the first failure; owned[i].res records what each
// delete reported, including a partial result, for the caller to record.
func deleteOwned(ctx context.Context, m *Manager, tx store.Store, owner string, owned []ownedDeletion) error {
	for i := range owned {
		res, err := tx.DeleteFamily(ctx, owned[i].id, true)
		owned[i].res = res
		if err != nil {
			return ownedDeleteError(owner, err)
		}
		// A face that appeared after the prepare step, as for the owner.
		aErr := m.authorizeFamily(ctx, acl.OpDelete, owned[i].id, res.DeletedEntities, owned[i].authorized)
		if aErr != nil {
			return ownedDeleteError(owner, aErr)
		}
	}
	return nil
}

// ownedDeleteError wraps a failure on an owned entity without its id. A
// denial keeps its type, so it still maps to a 403; its decision names types
// and roles only.
func ownedDeleteError(owner string, err error) error {
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) {
		return fmt.Errorf("cannot delete %s: an entity it owns cannot be deleted: %w",
			owner, &acl.ForbiddenError{Decision: fe.Decision})
	}
	return fmt.Errorf("cannot delete %s: an entity it owns could not be deleted: %w", owner, withoutDetail(err))
}

// ownedRestoreError is ownedDeleteError for a restore. A unique-property
// conflict keeps its type too, so it still maps to a validation error; it
// names only the property, which is config.
func ownedRestoreError(owner string, err error) error {
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) {
		return fmt.Errorf("cannot restore %s: an entity it owns cannot be restored: %w",
			owner, &acl.ForbiddenError{Decision: fe.Decision})
	}
	var up store.UniquePropertyError
	if errors.As(err, &up) {
		return fmt.Errorf("cannot restore %s: an entity it owns could not be restored: %w", owner, up)
	}
	return fmt.Errorf("cannot restore %s: an entity it owns could not be restored: %w", owner, withoutDetail(err))
}

// errOwnedFailure stands in for a failure on an owned entity whose text may
// name that entity or its neighbors, which the caller may not be able to read.
var errOwnedFailure = errors.New("internal error")

// withoutDetail logs err in full and returns an error without its text.
// Context errors pass through, so a cancelled request still reads as one.
func withoutDetail(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	slog.Error("entitymanager: failure on an owned entity", "err", err)
	return errOwnedFailure
}

// withoutRelationsOf drops from capture every relation touching one of the
// owned entities. Those were deleted, and are recorded, with the owned entity.
func withoutRelationsOf(capture *cascadeCapture, owned []ownedDeletion) {
	if capture == nil || len(owned) == 0 {
		return
	}
	gone := make(map[string]bool, len(owned))
	for _, o := range owned {
		gone[o.id] = true
	}
	keep := func(rels []*entity.Relation) []*entity.Relation {
		out := rels[:0]
		for _, r := range rels {
			if !gone[r.From] && !gone[r.To] {
				out = append(out, r)
			}
		}
		return out
	}
	capture.incoming = keep(capture.incoming)
	capture.outgoing = keep(capture.outgoing)
}

// ownerDeleteTrigger is the triggered_by label on every record an owner's
// delete writes for the entities it owns.
func ownerDeleteTrigger(owner string) string {
	return "cascade:owner-delete:" + owner
}

// recordOwnedDeletes writes the versions and audit records for owned
// entities deleted with owner, after the transaction committed. They carry
// the owner-delete label, so the log shows why they went.
func recordOwnedDeletes(ctx context.Context, m *Manager, owner string, owned []ownedDeletion) {
	tb := ownerDeleteTrigger(owner)
	ownedCtx := audit.WithTriggeredBy(ctx, tb)
	// An edge between two owned entities is in both captures.
	versioned := make(map[string]bool)
	for _, o := range owned {
		if o.res == nil {
			continue
		}
		for _, e := range o.res.DeletedEntities {
			m.recordEntityVersion(ownedCtx, store.VersionOpDelete, e, "")
		}
		for _, rel := range append(append([]*entity.Relation{}, o.capture.incoming...), o.capture.outgoing...) {
			if k := relationKey(rel); !versioned[k] {
				versioned[k] = true
				m.recordRelationVersion(ownedCtx, store.VersionOpDelete, rel, "", "", tb)
			}
		}
		for _, rel := range o.res.DeletedRelations {
			m.recordRelationAudit(ownedCtx, audit.OpDeleteRelation, rel, "deleted")
		}
		m.recordFamilyDeleteAudit(ownedCtx, o.res.DeletedEntities, len(o.res.DeletedRelations))
		m.notifyAliasesOfDelete(ctx, o.id)
	}
}

// coMarkWindow bounds how far apart an owner's soft-delete mark and an owned
// entity's mark may be for a restore of the owner to bring the owned entity
// back. Both are marked in one transaction, so in practice they are
// microseconds apart. The bound keeps a restore from reviving an owned entity
// that someone restored on its own and then deleted again later.
//
// The window is open on both sides. The owner is marked first, but fsstore
// keeps wall-clock time only, so a clock step between the two marks can put
// the owned entity's mark slightly before the owner's. An owned entity
// deleted on its own before its owner is not at risk from the earlier side:
// its mark then holds the owning edge, so the owner's hidden edges do not
// list it.
//
// Matching on deleter and time is a heuristic. An explicit link stored with
// the mark, naming the mark it was taken with, would be the robust fix.
const coMarkWindow = 5 * time.Second

// markedTogether reports whether child's mark was taken with owner's: by the
// same principal, within [coMarkWindow] of it either way.
func markedTogether(owner, child store.MarkedEntity) bool {
	if child.DeletedBy != owner.DeletedBy {
		return false
	}
	d := child.DeletedAt.Sub(owner.DeletedAt)
	return d >= -coMarkWindow && d <= coMarkWindow
}

// markedWithOwner returns the marked entities that owner's soft delete took
// along: targets of owning edges hidden with owner that were marked together
// with it (see markedTogether). marked indexes every marked family by id.
func markedWithOwner(
	ctx context.Context, m *Manager, tx store.Store, owner store.MarkedEntity, marked map[string]store.MarkedEntity,
) ([]store.MarkedEntity, error) {
	outgoing, err := collectIncidentRelations(store.WithRevealed(ctx, owner.ID), tx, owner.ID, store.DirectionOutgoing)
	if err != nil {
		return nil, fmt.Errorf("collect owned entities of %q: %w", owner.ID, err)
	}
	var out []store.MarkedEntity
	for _, id := range ownedChildIDs(m.deps.Meta, owner.ID, outgoing) {
		me, ok := marked[id]
		if ok && len(me.Entities) > 0 && markedTogether(owner, me) {
			out = append(out, me)
		}
	}
	return out, nil
}

// checkRestoredOwning refuses a restore that would bring back an owning edge
// the live graph no longer admits. It checks every hidden owning edge that
// comes back with the restore and has its other end live: while the restored
// entities were marked, that end may have gained an owner or an owned entity
// of its own. An edge between two restored entities needs no check: neither
// end could gain an edge while marked.
//
// The refusal does not name the edge's other end, for the reason
// owningConflict gives.
func checkRestoredOwning(
	ctx context.Context, m *Manager, tx store.Store, restoring []string, marked map[string]store.MarkedEntity,
) error {
	revived, err := revivedOwningEdges(ctx, m, tx, restoring, marked)
	if err != nil {
		return err
	}
	for _, key := range revived {
		// Checked under ctx, which reveals nothing: against the live graph.
		cErr := CheckOwningEdge(ctx, m.deps.Meta, tx, key)
		if errors.Is(cErr, ErrOwningRule) {
			return fmt.Errorf("%w: restoring %s would bring back an owning edge that breaks ownership: "+
				"an entity has at most one owner, and an owned entity owns nothing", ErrOwningRule, restoring[0])
		}
		if cErr != nil {
			return cErr
		}
	}
	return nil
}

// revivedOwningEdges lists the hidden owning edges of restoring that come
// back with the restore and have their other end live. marked holds the
// families that stay marked.
func revivedOwningEdges(
	ctx context.Context, m *Manager, tx store.Store, restoring []string, marked map[string]store.MarkedEntity,
) ([]entity.RelationKey, error) {
	inRestore := make(map[string]bool, len(restoring))
	for _, id := range restoring {
		inRestore[id] = true
	}
	revealed := store.WithRevealed(ctx, restoring...)
	var out []entity.RelationKey
	for _, id := range restoring {
		for _, dir := range []store.Direction{store.DirectionIncoming, store.DirectionOutgoing} {
			rels, err := collectIncidentRelations(revealed, tx, id, dir)
			if err != nil {
				return nil, err
			}
			for _, rel := range rels {
				if !metamodel.IsOwning(m.deps.Meta, rel.Type) || (inRestore[rel.From] && inRestore[rel.To]) {
					continue
				}
				if _, stillMarked := marked[otherEndOf(rel, id)]; stillMarked {
					continue // stays hidden, and follows the other end
				}
				out = append(out, rel.Identity())
			}
		}
	}
	return out, nil
}

// otherEndOf returns the endpoint of rel that is not id.
func otherEndOf(rel *entity.Relation, id string) string {
	if rel.From == id {
		return rel.To
	}
	return rel.From
}
