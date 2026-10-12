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
	// BeforeDelete runs before the delete's transaction. It returns
	// [*AnchorBlockedError] when a channel of a `block` type would go, and
	// otherwise detaches the channels that go. The returned undo puts them
	// back when the transaction then fails; nil means nothing to undo.
	BeforeDelete(ctx context.Context, d AnchorDelete) (undo func(context.Context) error, err error)
	// CheckDelete is the block check of BeforeDelete without the detach. A
	// soft delete calls it: the rows stay restorable, so the channels stay.
	// It never writes.
	CheckDelete(ctx context.Context, d AnchorDelete) error
	// BeforeRename detaches channels left on newID by an earlier entity, so
	// the renamed entity does not inherit them. It calls free before it
	// writes anything; free returns [ErrEntityAlreadyExists] when newID is
	// taken, and the guard then returns that error untouched. A guard that
	// finds no channels on newID need not call free.
	BeforeRename(ctx context.Context, newID string, free func(context.Context) error) error
}

// AnchorDelete describes what one delete removes, for [AnchorGuard].
type AnchorDelete struct {
	// ID is the entity id.
	ID string
	// Face is the face being deleted, or empty for the whole entity.
	Face entity.Face
	// Block reports whether channels anchored to the whole entity count for
	// a block. It is true when the caller's last readable face goes, so the
	// answer never depends on a face the caller cannot read. The channels of
	// Face itself always count.
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

// BeforeDelete implements [AnchorGuard] by allowing everything.
func (NopAnchorGuard) BeforeDelete(context.Context, AnchorDelete) (func(context.Context) error, error) {
	return nil, nil //nolint:nilnil // a nil undo with no error is "nothing to undo", as the interface documents
}

// CheckDelete implements [AnchorGuard] by allowing everything.
func (NopAnchorGuard) CheckDelete(context.Context, AnchorDelete) error { return nil }

// BeforeRename implements [AnchorGuard] by allowing everything.
func (NopAnchorGuard) BeforeRename(context.Context, string, func(context.Context) error) error {
	return nil
}

// errFacesChanged reports a face delete whose last-face answer changed
// between the guard and the transaction. The guard acted on the old answer,
// so the delete is refused and can be retried.
var errFacesChanged = fmt.Errorf("delete face: the entity's faces changed during the delete; retry: %w",
	store.ErrConflict)

// The functions below take *Manager rather than being methods on it, for the
// reason softdelete.go gives: Manager sits on its plimsoll method load line.

// guarded reports whether m has a guard that can act. NopAnchorGuard cannot,
// so the authorization pre-pass that only serves the guard is skipped.
func guarded(m *Manager) bool {
	_, nop := m.deps.AnchorGuard.(NopAnchorGuard)
	return !nop
}

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

// anchorUndo collects the undo functions of the guard calls one operation
// made.
type anchorUndo []namedUndo

type namedUndo struct {
	id   string
	face entity.Face
	fn   func(context.Context) error
}

// guardDeletes calls BeforeDelete for each of ds in order. When one refuses,
// the detaches already made are undone and its error is returned.
func guardDeletes(ctx context.Context, g AnchorGuard, ds ...AnchorDelete) (anchorUndo, error) {
	var undo anchorUndo
	for _, d := range ds {
		fn, err := g.BeforeDelete(ctx, d)
		if err != nil {
			undo.run(ctx, func(string, entity.Face) bool { return true })
			return nil, err
		}
		if fn != nil {
			undo = append(undo, namedUndo{id: d.ID, face: d.Face, fn: fn})
		}
	}
	return undo, nil
}

// run calls every undo whose anchor stillStored reports, newest first. A
// failed undo is logged: the channels stay detached, which hides them but
// loses nothing, and the operation's own error is what the caller reports.
func (u anchorUndo) run(ctx context.Context, stillStored func(id string, face entity.Face) bool) {
	for i := len(u) - 1; i >= 0; i-- {
		n := u[i]
		if !stillStored(n.id, n.face) {
			continue
		}
		if err := n.fn(ctx); err != nil {
			slog.Error("entitymanager: cannot restore the channels detached for a failed delete",
				"id", n.id, "face", n.face, "error", err)
		}
	}
}

// undoFailedDelete restores the detached channels of every entity a failed
// delete left in place. A family or face that is gone stays detached: on a
// backend that cannot roll back, the delete happened.
func undoFailedDelete(ctx context.Context, m *Manager, undo anchorUndo) {
	undo.run(ctx, func(id string, face entity.Face) bool {
		if face == "" {
			return familyStillStored(ctx, m.deps.Store, id)
		}
		_, err := m.deps.Store.GetEntity(ctx, entity.Ref{ID: id, Face: face})
		return !errors.Is(err, store.ErrNotFound)
	})
}

// guardFamilyDelete runs the pre-pass and the guard for a whole-family
// delete of id and of the owned entities it takes along. It returns the
// undo for a failed transaction, and the owned ids it guarded so the
// transaction can refuse an owned entity that appeared since. Without a
// guard it does nothing and returns a nil set, which the transaction reads
// as "do not check".
func guardFamilyDelete(
	ctx context.Context, m *Manager, id string, cascade bool,
) (anchorUndo, map[string]bool, error) {
	if !guarded(m) {
		return nil, nil, nil
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
	undo, err := guardDeletes(ctx, m.deps.AnchorGuard, ds...)
	if err != nil {
		return nil, nil, err
	}
	return undo, ownedSet, nil
}

// requireGuardedOwned refuses an owned delete the guard did not see. An
// owning edge added between the pre-pass and the transaction would
// otherwise take an entity along without its channels being checked. A nil
// set means no guard ran.
func requireGuardedOwned(guardedOwned map[string]bool, owned []ownedDeletion) error {
	if guardedOwned == nil {
		return nil
	}
	for _, o := range owned {
		if !guardedOwned[o.id] {
			return fmt.Errorf("delete: the entities it owns changed during the delete; retry: %w",
				store.ErrConflict)
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
// changed, and the undo for a failed transaction.
type faceDeleteGuard struct {
	callerLast, storedLast bool
	undo                   anchorUndo
	active                 bool
}

// guardFaceDelete runs the pre-pass and the guard for a delete of id@face.
// The pre-pass reads the edges the transaction will check, against the outer
// store, and gives the transaction's refusals first.
func guardFaceDelete(
	ctx context.Context, m *Manager, id string, face entity.Face, cascade bool,
	readableSiblings map[entity.Face]bool,
) (faceDeleteGuard, error) {
	if !guarded(m) {
		return faceDeleteGuard{}, nil
	}
	pctx := prepassContext(ctx, m)
	incoming, outgoing, callerLast, storedLast, err := faceDeleteEdges(pctx, m.deps.Store, id, face, readableSiblings)
	if err != nil {
		return faceDeleteGuard{}, err
	}
	if eErr := checkFaceDeleteEdges(pctx, m, m.deps.Store, id, cascade, callerLast, incoming, outgoing); eErr != nil {
		return faceDeleteGuard{}, eErr
	}
	undo, err := guardDeletes(ctx, m.deps.AnchorGuard,
		AnchorDelete{ID: id, Face: face, Block: callerLast, Detach: storedLast})
	if err != nil {
		return faceDeleteGuard{}, err
	}
	return faceDeleteGuard{callerLast: callerLast, storedLast: storedLast, undo: undo, active: true}, nil
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
// before it writes. It refuses when the last-face answers differ from the
// ones the guard acted on.
func (g faceDeleteGuard) txEdges(
	ctx context.Context, m *Manager, tx store.Store, id string, face entity.Face, cascade bool,
	readableSiblings map[entity.Face]bool,
) (incoming, outgoing []*entity.Relation, storedLast bool, err error) {
	incoming, outgoing, callerLast, storedLast, err := faceDeleteEdges(ctx, tx, id, face, readableSiblings)
	if err != nil {
		return nil, nil, false, err
	}
	if g.active && (callerLast != g.callerLast || storedLast != g.storedLast) {
		return nil, nil, false, errFacesChanged
	}
	if cErr := checkFaceDeleteEdges(ctx, m, tx, id, cascade, callerLast, incoming, outgoing); cErr != nil {
		return nil, nil, false, cErr
	}
	return incoming, outgoing, storedLast, nil
}

// checkSoftDelete runs the soft delete's block check for id and the owned
// entities it marks with it. The pre-pass gives the transaction's refusals
// first, so a 403 is never preceded by a 409.
func checkSoftDelete(ctx context.Context, m *Manager, id string) error {
	if !guarded(m) {
		return nil
	}
	owned, err := deletePrepass(ctx, m, id, true)
	if err != nil {
		return err
	}
	for _, target := range append([]string{id}, owned...) {
		if err := m.deps.AnchorGuard.CheckDelete(ctx,
			AnchorDelete{ID: target, Block: true, Detach: true}); err != nil {
			return err
		}
	}
	return nil
}

// guardPurge detaches the channels of a soft-deleted id before its purge.
func guardPurge(ctx context.Context, m *Manager, id string) (anchorUndo, error) {
	return guardDeletes(ctx, m.deps.AnchorGuard, AnchorDelete{ID: id, Block: true, Detach: true, Purge: true})
}

// guardRename runs the rename guard for newID. free applies the stores' "id
// taken" rule, so channels on newID are detached only when the rename can
// go ahead, and a rename onto a taken id leaves the occupant's channels
// alone.
func guardRename(ctx context.Context, m *Manager, oldID, newID string) error {
	// The store refuses a malformed id anyway; refusing it here keeps it
	// away from the channel store, which would report it as its own error.
	if err := entity.ValidateID(newID); err != nil {
		return fmt.Errorf("invalid new ID: %w", err)
	}
	return m.deps.AnchorGuard.BeforeRename(ctx, newID, func(ctx context.Context) error {
		return requireIDFree(ctx, m.deps.Store, newID, oldID)
	})
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
