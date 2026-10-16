package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AnchorGuard applies a channel type's `on_entity_delete` rule before a
// delete or rename writes anything, so a `block` refuses the write instead
// of finding out after it. Realtime channels hang off entity ids (see
// internal/channels); the guard is how the write path consults them without
// importing them. Defined here, at its only call site (CLAUDE.md
// consumer-side interfaces); appbuild wires the implementation.
//
// The post-commit alias hooks stay the second line of defense: they sweep
// channels a concurrent first post created while the write ran.
type AnchorGuard interface {
	// Active reports whether the guard can act at all. When it is false the
	// manager skips every guard step, including the authorization pre-pass
	// that only serves the guard. It is an explicit capability rather than a
	// type check, so a wrapped or pointer [NopAnchorGuard] stays inactive.
	Active() bool
	// Scope starts the guard's part in one delete that removes ids. It
	// reads which of ids have channels, and the policy the operation is
	// judged under, once: every [AnchorScope] call of the operation sees the
	// same configuration.
	//
	// Nil: returned with a nil error when none of ids has a channel; the
	// manager then skips the pre-pass and the guard.
	Scope(ctx context.Context, ids []string) (AnchorScope, error)
	// BeforeRename detaches channels left on newID by an earlier entity, so
	// the renamed entity does not inherit them. It calls free before it
	// writes anything; free returns [ErrEntityAlreadyExists] when newID is
	// taken, and the guard then returns that error untouched. A guard that
	// finds no channels on newID need not call free. The returned undo puts
	// the detached channels back when the rename then fails.
	//
	// Nil: the undo is nil when nothing was detached.
	BeforeRename(ctx context.Context, newID string, free func(context.Context) error) (AnchorUndo, error)
}

// AnchorScope is the guard's view of one delete operation (see
// [AnchorGuard.Scope]).
type AnchorScope interface {
	// BeforeDelete runs before the delete's transaction. It returns
	// [*AnchorBlockedError] when a channel of a `block` type would go, and
	// otherwise detaches the channels that go. The returned undo puts them
	// back when the transaction then fails. When it returns an error it has
	// detached nothing: a detach that failed part way is put back first.
	//
	// Nil: the undo is nil when nothing was detached.
	BeforeDelete(ctx context.Context, d AnchorDelete) (AnchorUndo, error)
	// CheckDelete is the block check of BeforeDelete without the detach. A
	// soft delete calls it: the rows stay restorable, so the channels stay.
	// It never writes.
	CheckDelete(ctx context.Context, d AnchorDelete) error
}

// AnchorUndo puts back the channels one guard call detached.
type AnchorUndo interface {
	// Restore puts the channels back. It refuses, and restores nothing,
	// when live channels now sit where they would go.
	Restore(ctx context.Context) error
	// String names the detached batch so an operator can restore it by
	// hand when the manager cannot. It carries ids, never entity content.
	String() string
}

// AnchorDelete describes what one delete removes, for [AnchorScope].
type AnchorDelete struct {
	// ID is the entity id.
	ID string
	// Face is the face being deleted, or empty for the whole entity.
	Face entity.Face
	// Block reports whether channels anchored to the whole entity count for
	// a block. It is true when the caller's last readable face goes, so the
	// answer never depends on a face the caller cannot read. The channels of
	// Face itself always count.
	//
	// A whole-family delete always sets it. That keeps the hidden-face
	// property only because callers resolve a bare id through
	// visibility.Resolver.WriteTarget before they delete: the dataentry
	// handler refuses a bare id on a faced type as face_required, so a
	// family with a face the caller cannot read never reaches
	// [Manager.DeleteEntity] from it. A surface that passed an unresolved
	// bare id would let a channel the caller cannot see answer the block.
	Block bool
	// Detach reports whether the entity really goes, which is when its last
	// stored face goes: every channel of the id is detached. Otherwise only
	// the channels of Face are.
	Detach bool
	// Purge marks the purge of a soft-deleted entity. Its delete was decided
	// when it was soft-deleted, so a block no longer refuses: the guard
	// detaches the channels and logs a warning.
	Purge bool
}

// AnchorBlockedError reports a delete refused because a channel of
// ChannelType is attached to the entity and its type says `on_entity_delete:
// block`. The surfaces map it to 409. Naming the type tells the caller that a
// channel of that type exists on an entity it may delete; that one bit is
// accepted (docs/acl-security.md).
type AnchorBlockedError struct {
	ChannelType string
}

func (e *AnchorBlockedError) Error() string {
	return fmt.Sprintf("cannot delete: a %s channel is attached and its type blocks the delete", e.ChannelType)
}

// NopAnchorGuard guards nothing. It is the explicit opt-out for a build or
// test with no realtime channels; never the result of leaving
// [Deps.AnchorGuard] nil.
type NopAnchorGuard struct{}

// Active implements [AnchorGuard]: a NopAnchorGuard never acts.
func (NopAnchorGuard) Active() bool { return false }

// Scope implements [AnchorGuard] by reporting no channels.
func (NopAnchorGuard) Scope(context.Context, []string) (AnchorScope, error) {
	return nil, nil //nolint:nilnil // a nil scope is "no channels", as the interface documents
}

// BeforeRename implements [AnchorGuard] by allowing everything.
func (NopAnchorGuard) BeforeRename(context.Context, string, func(context.Context) error) (AnchorUndo, error) {
	return nil, nil //nolint:nilnil // a nil undo is "nothing detached", as the interface documents
}

// errFacesChanged reports a face delete whose last-face answer changed
// between the guard and the transaction. The guard acted on the old answer,
// so the delete is refused and can be retried.
var errFacesChanged = fmt.Errorf("delete face: the entity's faces changed during the delete; retry: %w",
	store.ErrConflict)

// errOwnedChanged reports an owned entity the guard did not see, refused for
// the same reason as errFacesChanged.
var errOwnedChanged = fmt.Errorf("delete: the entities it owns changed during the delete; retry: %w",
	store.ErrConflict)

// The functions below take *Manager rather than being methods on it, for the
// reason softdelete.go gives: Manager sits on its plimsoll method load line.

// prepassContext is the context of an authorization pre-pass. It repeats
// checks the transaction makes again, so on an elevated handle the acl-bypass
// records it would write are suppressed: the transaction writes them once. A
// denial is still recorded, because it ends the operation.
func prepassContext(ctx context.Context, m *Manager) context.Context {
	if m.bypassACL {
		return withAffordanceProbe(ctx)
	}
	return ctx
}

// deletePrepass runs, outside the transaction, the checks deleteEntityInTx
// makes before it writes: the cascade opt-in, the cascade relation grants
// and the owned deletes. A refusal it finds is the refusal the transaction
// would give, so it comes before the guard and no channel is detached for a
// delete that was never going to happen. It returns the ids of the owned
// entities the cascade takes along.
func deletePrepass(ctx context.Context, m *Manager, id string, cascade bool) ([]string, error) {
	ctx = prepassContext(ctx, m)
	st := m.deps.Store
	incoming, err := collectIncidentRelations(ctx, st, id, store.DirectionIncoming)
	if err != nil {
		return nil, fmt.Errorf("collect incoming relations for %q: %w", id, err)
	}
	outgoing, err := collectIncidentRelations(ctx, st, id, store.DirectionOutgoing)
	if err != nil {
		return nil, fmt.Errorf("collect outgoing relations for %q: %w", id, err)
	}
	if len(incoming)+len(outgoing) == 0 {
		return nil, nil
	}
	if !cascade {
		return nil, ErrHasRelations
	}
	if aErr := m.authorizeCascadeRelations(ctx, st, id, nil, incoming, outgoing); aErr != nil {
		return nil, aErr
	}
	owned, err := prepareOwnedDeletes(ctx, m, st, id, outgoing)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(owned))
	for i, o := range owned {
		ids[i] = o.id
	}
	return ids, nil
}

// anchorCandidates returns id and the entities a cascade delete of id could
// take along: the targets of its owning edges. It authorizes nothing and is
// one relation read, so it is cheap enough to run before knowing whether any
// of them has channels.
func anchorCandidates(ctx context.Context, m *Manager, id string) ([]string, error) {
	outgoing, err := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionOutgoing)
	if err != nil {
		return nil, fmt.Errorf("collect outgoing relations for %q: %w", id, err)
	}
	return append([]string{id}, ownedChildIDs(m.deps.Meta, id, outgoing)...), nil
}

// openScope asks the guard whether any of ids has channels. A nil scope
// means none does, or the guard is inactive, and the caller skips the
// pre-pass and the guard. ids[0] is the entity the caller addressed; an
// error over several ids may name an owned one, so it is logged and
// replaced.
func openScope(ctx context.Context, m *Manager, ids []string) (AnchorScope, error) {
	g := m.deps.AnchorGuard
	if !g.Active() {
		return nil, nil //nolint:nilnil // no guard: nothing to scope
	}
	scope, err := g.Scope(ctx, ids)
	if err != nil && len(ids) > 1 {
		return nil, fmt.Errorf("cannot delete %s: cannot read the channels of what it removes: %w",
			ids[0], withoutDetail(err))
	}
	return scope, err
}

// ownedGuardError wraps a guard failure on an entity owner's delete takes
// along, without the owned id: the caller addressed only owner. A block
// keeps its type, so it still maps to 409; it names only the channel type,
// which is config. Any other error is logged in full and replaced, as
// ownedDeleteError does.
func ownedGuardError(owner string, err error) error {
	var blocked *AnchorBlockedError
	if errors.As(err, &blocked) {
		return fmt.Errorf("cannot delete %s: an entity it owns has a channel that blocks the delete: %w",
			owner, blocked)
	}
	return ownedDeleteError(owner, err)
}

// anchorUndo collects the undos of the guard calls one operation made.
type anchorUndo []namedUndo

type namedUndo struct {
	id   string
	face entity.Face
	undo AnchorUndo
}

// guardDeletes calls BeforeDelete for each of ds in order. When one refuses,
// the detaches already made are undone and its error is returned. owner is
// the id the caller addressed; an error on any other id is wrapped by
// ownedGuardError.
func guardDeletes(ctx context.Context, scope AnchorScope, owner string, ds ...AnchorDelete) (anchorUndo, error) {
	var undo anchorUndo
	for _, d := range ds {
		u, err := scope.BeforeDelete(ctx, d)
		if err != nil {
			undo.restore(ctx, nil, nil)
			if d.ID != owner {
				return nil, ownedGuardError(owner, err)
			}
			return nil, err
		}
		if u != nil {
			undo = append(undo, namedUndo{id: d.ID, face: d.Face, undo: u})
		}
	}
	return undo, nil
}

// restore calls Restore on every undo whose anchor stillStored reports,
// newest first, and gone on the others. A nil stillStored restores all.
//
// It runs on a context the request's cancellation does not reach, bounded
// like an alias hook (aliasHookContext): the write it undoes has already
// failed, and a client that disconnects must not leave channels detached
// from an entity that is still there.
//
// A failed restore is logged with the batch: the channels stay detached,
// which hides them but loses nothing, and the operation's own error is what
// the caller reports.
func (u anchorUndo) restore(
	ctx context.Context, stillStored func(ctx context.Context, id string, face entity.Face) bool,
	gone func(namedUndo),
) {
	if len(u) == 0 {
		return
	}
	ctx, cancel := aliasHookContext(ctx)
	defer cancel()
	for i := len(u) - 1; i >= 0; i-- {
		n := u[i]
		if stillStored != nil && !stillStored(ctx, n.id, n.face) {
			if gone != nil {
				gone(n)
			}
			continue
		}
		if err := n.undo.Restore(ctx); err != nil {
			slog.Error("entitymanager: cannot restore the channels detached for a failed write",
				"id", n.id, "face", n.face, "tombstone", n.undo.String(), "error", err)
		}
	}
}

// undoFailedDelete restores the detached channels of every entity a failed
// delete left in place. A family or face that is gone stays detached: on a
// backend that cannot roll back, the delete happened.
//
// One case strands them: the delete failed as not-found because the family
// went meanwhile, most likely renamed by a concurrent request. Its channels
// then sit tombstoned under the old id while the entity lives on under the
// new one. That is logged at warn with the batch, so an operator can put
// them back.
func undoFailedDelete(ctx context.Context, m *Manager, undo anchorUndo, txErr error) {
	notFound := errors.Is(txErr, ErrEntityNotFound) || errors.Is(txErr, store.ErrNotFound)
	undo.restore(ctx, func(ctx context.Context, id string, face entity.Face) bool {
		if face == "" {
			return familyStillStored(ctx, m.deps.Store, id)
		}
		_, err := m.deps.Store.GetEntity(ctx, entity.Ref{ID: id, Face: face})
		return !errors.Is(err, store.ErrNotFound)
	}, func(n namedUndo) {
		if notFound {
			slog.Warn("entitymanager: a delete found its entity gone, possibly renamed meanwhile; "+
				"its channels stay detached under the old id",
				"id", n.id, "face", n.face, "tombstone", n.undo.String())
		}
	})
}

// guardFamilyDelete runs the pre-pass and the guard for a whole-family
// delete of id and of the owned entities it takes along. It returns the
// undo for a failed transaction, and the owned ids it guarded so the
// transaction can refuse an owned entity that appeared since.
//
// When none of those entities has channels, it skips the pre-pass. When the
// guard detached nothing, it returns a nil set, which the transaction reads
// as "do not check": there is nothing for a changed owned set to make
// inconsistent, and a channel created meanwhile is the post-commit sweep's.
func guardFamilyDelete(
	ctx context.Context, m *Manager, id string, cascade bool,
) (anchorUndo, map[string]bool, error) {
	if !m.deps.AnchorGuard.Active() {
		return nil, nil, nil
	}
	ids, err := anchorCandidates(ctx, m, id)
	if err != nil {
		return nil, nil, err
	}
	scope, err := openScope(ctx, m, ids)
	if err != nil || scope == nil {
		return nil, nil, err
	}
	owned, err := deletePrepass(ctx, m, id, cascade)
	if err != nil {
		return nil, nil, err
	}
	ds := []AnchorDelete{{ID: id, Block: true, Detach: true}}
	ownedSet := make(map[string]bool, len(owned))
	for _, o := range owned {
		ownedSet[o] = true
		ds = append(ds, AnchorDelete{ID: o, Block: true, Detach: true})
	}
	undo, err := guardDeletes(ctx, scope, id, ds...)
	if err != nil {
		return nil, nil, err
	}
	if len(undo) == 0 {
		return nil, nil, nil
	}
	return undo, ownedSet, nil
}

// requireGuardedOwned refuses an owned delete the guard did not see. An
// owning edge added between the pre-pass and the transaction would
// otherwise take an entity along without its channels being checked, while
// the channels the guard did detach could no longer be put back as one
// batch. A nil set means the guard detached nothing.
func requireGuardedOwned(guardedOwned map[string]bool, owned []ownedDeletion) error {
	if guardedOwned == nil {
		return nil
	}
	for _, o := range owned {
		if !guardedOwned[o.id] {
			return errOwnedChanged
		}
	}
	return nil
}

// faceLastness reports, from the stored faces of one family, whether face is
// its last stored face and whether it is the last face the caller may read.
// readableSiblings holds the faces other than face that the caller may read.
func faceLastness(
	headers []store.EntityHeader, face entity.Face, readableSiblings map[entity.Face]bool,
) (callerLast, storedLast bool) {
	storedLast = len(headers) == 1
	callerLast = true
	for _, h := range headers {
		if h.Face != face && readableSiblings[h.Face] {
			callerLast = false
		}
	}
	return callerLast, storedLast
}

// faceDeleteGuard is the guard step of one DeleteEntityFace: the last-face
// answers the guard acted on, so the transaction can refuse when they
// changed, and the undo for a failed transaction. active is set only when
// the guard detached something; otherwise there is nothing a changed answer
// could make inconsistent.
type faceDeleteGuard struct {
	callerLast, storedLast bool
	undo                   anchorUndo
	active                 bool
}

// guardFaceDelete runs the pre-pass and the guard for a delete of id@face.
// The pre-pass reads the edges the transaction will check, against the outer
// store, and gives the transaction's refusals first. It is skipped when id
// has no channels.
func guardFaceDelete(
	ctx context.Context, m *Manager, id string, face entity.Face, cascade bool,
	readableSiblings map[entity.Face]bool,
) (faceDeleteGuard, error) {
	scope, err := openScope(ctx, m, []string{id})
	if err != nil || scope == nil {
		return faceDeleteGuard{}, err
	}
	pctx := prepassContext(ctx, m)
	incoming, outgoing, callerLast, storedLast, err := faceDeleteEdges(pctx, m.deps.Store, id, face, readableSiblings)
	if err != nil {
		return faceDeleteGuard{}, err
	}
	if eErr := checkFaceDeleteEdges(pctx, m, m.deps.Store, id, cascade, callerLast, incoming, outgoing); eErr != nil {
		return faceDeleteGuard{}, eErr
	}
	undo, err := guardDeletes(ctx, scope, id,
		AnchorDelete{ID: id, Face: face, Block: callerLast, Detach: storedLast})
	if err != nil {
		return faceDeleteGuard{}, err
	}
	return faceDeleteGuard{callerLast: callerLast, storedLast: storedLast, undo: undo, active: len(undo) > 0}, nil
}

// checkFaceDeleteEdges makes the checks a face delete makes on the edges it
// cuts, before it writes. The last face removes the entity, so it takes the
// same opt-in a family delete does (RR-2466U1). "Last" is the caller's view,
// so a hidden sibling cannot change the answer. Every cut edge is authorized
// like a cascade.
func checkFaceDeleteEdges(
	ctx context.Context, m *Manager, st store.Store, id string, cascade, callerLast bool,
	incoming, outgoing []*entity.Relation,
) error {
	if len(incoming)+len(outgoing) == 0 {
		return nil
	}
	if callerLast && !cascade {
		return ErrHasRelations
	}
	return m.authorizeCascadeRelations(ctx, st, id, nil, incoming, outgoing)
}

// txEdges reads, under tx, the edges a delete of id@face cuts and whether
// the face is the family's last, and makes the checks the transaction makes
// before it writes. It then refuses when the last-face answers differ from
// the ones the guard acted on. The checks come first, so a caller who would
// be refused never sees the retryable conflict.
func (g faceDeleteGuard) txEdges(
	ctx context.Context, m *Manager, tx store.Store, id string, face entity.Face, cascade bool,
	readableSiblings map[entity.Face]bool,
) (incoming, outgoing []*entity.Relation, storedLast bool, err error) {
	incoming, outgoing, callerLast, storedLast, err := faceDeleteEdges(ctx, tx, id, face, readableSiblings)
	if err != nil {
		return nil, nil, false, err
	}
	if cErr := checkFaceDeleteEdges(ctx, m, tx, id, cascade, callerLast, incoming, outgoing); cErr != nil {
		return nil, nil, false, cErr
	}
	if g.active && (callerLast != g.callerLast || storedLast != g.storedLast) {
		return nil, nil, false, errFacesChanged
	}
	return incoming, outgoing, storedLast, nil
}

// checkSoftDelete runs the soft delete's block check for id and the owned
// entities it marks with it. The pre-pass gives the transaction's refusals
// first, so a 403 is never preceded by a 409. Both are skipped when none of
// them has channels.
func checkSoftDelete(ctx context.Context, m *Manager, id string) error {
	if !m.deps.AnchorGuard.Active() {
		return nil
	}
	ids, err := anchorCandidates(ctx, m, id)
	if err != nil {
		return err
	}
	scope, err := openScope(ctx, m, ids)
	if err != nil || scope == nil {
		return err
	}
	owned, err := deletePrepass(ctx, m, id, true)
	if err != nil {
		return err
	}
	for _, target := range append([]string{id}, owned...) {
		if err := scope.CheckDelete(ctx, AnchorDelete{ID: target, Block: true, Detach: true}); err != nil {
			if target != id {
				return ownedGuardError(id, err)
			}
			return err
		}
	}
	return nil
}

// guardPurge detaches the channels of a soft-deleted id before its purge.
func guardPurge(ctx context.Context, m *Manager, id string) (anchorUndo, error) {
	scope, err := openScope(ctx, m, []string{id})
	if err != nil || scope == nil {
		return nil, err
	}
	return guardDeletes(ctx, scope, id, AnchorDelete{ID: id, Block: true, Detach: true, Purge: true})
}

// guardRename runs the rename guard for newID. free applies the stores' "id
// taken" rule, so channels on newID are detached only when the rename can
// go ahead, and a rename onto a taken id leaves the occupant's channels
// alone. It returns the undo for a failed rename.
func guardRename(ctx context.Context, m *Manager, oldID, newID string) (anchorUndo, error) {
	// The store refuses a malformed id anyway; refusing it here keeps it
	// away from the channel store, which would report it as its own error.
	if err := entity.ValidateID(newID); err != nil {
		return nil, fmt.Errorf("invalid new ID: %w", err)
	}
	if !m.deps.AnchorGuard.Active() {
		return nil, nil
	}
	u, err := m.deps.AnchorGuard.BeforeRename(ctx, newID, func(ctx context.Context) error {
		return requireIDFree(ctx, m.deps.Store, newID, oldID)
	})
	if err != nil || u == nil {
		return nil, err
	}
	return anchorUndo{{id: newID, undo: u}}, nil
}

// undoFailedRename puts back the channels the guard detached from newID,
// unless the rename happened anyway (fs and memstore cannot roll back) and
// the renamed entity now holds newID.
func undoFailedRename(ctx context.Context, m *Manager, undo anchorUndo) {
	undo.restore(ctx, func(ctx context.Context, newID string, _ entity.Face) bool {
		return !familyStillStored(ctx, m.deps.Store, newID)
	}, nil)
}

// requireIDFree returns [ErrEntityAlreadyExists] when id is taken by an
// entity other than except: a live one, or a soft-deleted one, which keeps
// its id until it is purged. Ids are compared case-folded, as the stores
// compare them.
func requireIDFree(ctx context.Context, st store.Store, id, except string) error {
	taken, err := idTakenByOther(ctx, st, id, except)
	if err != nil {
		return err
	}
	if !taken {
		taken, err = markedIDTaken(ctx, st, id, except)
		if err != nil {
			return err
		}
	}
	if taken {
		return fmt.Errorf("%w: %s", ErrEntityAlreadyExists, id)
	}
	return nil
}

// markedIDTaken reports whether a soft-deleted entity other than except holds
// id under case folding. A store without soft delete has none.
func markedIDTaken(ctx context.Context, st store.Store, id, except string) (bool, error) {
	sd, ok := st.(store.SoftDeleteProvider)
	if !ok {
		return false, nil
	}
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return false, err
	}
	folded, exceptFolded := strings.ToLower(id), strings.ToLower(except)
	for _, me := range marked {
		if f := strings.ToLower(me.ID); f == folded && f != exceptFolded {
			return true, nil
		}
	}
	return false, nil
}
