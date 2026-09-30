package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RecreateEntity brings a deleted row back at its own id and face: the
// history restore of a deleted face (BUG-4SYAA6). The caller owns the complete
// entity state (id, type, face, properties, content) and RecreateEntity
// persists exactly that, with the ACL + validation + audit framing of the
// normal write path but WITHOUT running automation or the cascade.
//
// How it differs from [Manager.CreateEntity]:
//
//   - It does NOT reject an explicit ID for a non-manual id_type, generate an
//     ID, or apply a template / status default. There is no backfill, because
//     automation is suppressed.
//   - It runs NO automation and NO cascade: whatever the original create
//     derived already exists or was deleted on its own terms.
//   - It does NOT apply the state machines' entry rule: the row may come back
//     at any state the principal could have reached by declared transitions
//     (BUG-KK1UXH).
//
// It is create-only. A row already at that address is
// [ErrEntityAlreadyExists], never an update: the caller decided "deleted" from
// an earlier read, and a face recreated since then must not be overwritten by
// a whole-record write that skipped the per-field write gate. The durable
// write is a direct CreateEntity, so a concurrent create that lands between
// the probe and the write is rejected the same way (BUG-ZWTDH9).
//
// The existence probe runs before authorization on purpose, so an existing
// face is refused without an ACL check. That answer is not an existence
// oracle: every caller has already gated the history read that named the id.
//
// Validation includes the metamodel's ID-prefix check, which is a HARD
// error, so an ID matching no declared prefix is refused.
//
// It is a package function rather than a method for the reason given on
// [CopiesForSource]: Manager's method count is pinned. [Recreator] adapts it
// for consumers that need a method.
func RecreateEntity(ctx context.Context, m *Manager, e *entity.Entity) (*entity.UpdateResult, error) {
	ctx = withStoreAttribution(ctx)
	if e == nil {
		return nil, errors.New("entitymanager: RecreateEntity: entity is nil")
	}
	if e.ID == "" {
		return nil, errors.New("entitymanager: RecreateEntity: entity ID is empty")
	}
	if e.IsLocked() {
		// The in-memory entity has redacted fields; writing it would replace
		// the on-disk content (typically ciphertext) with the cleartext shell.
		// Same guard the rest of the write path applies.
		return nil, fmt.Errorf("entitymanager: RecreateEntity: entity %s has inaccessible fields", e.ID)
	}

	// The probe addresses the row the body NAMES, not the zero coordinate: a
	// type declaring faces has no zero-face row (BUG-HC6I2T).
	//
	// It fails CLOSED: a non-nil error that is NOT [store.ErrNotFound] (a
	// flaky backend, a cancelled context) is returned rather than being
	// treated as "does not exist", so a transient read failure never becomes
	// a create against a row that is really there.
	_, getErr := m.deps.Store.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face})
	switch {
	case getErr == nil:
		return nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, entity.FormatStateRef(e.ID, e.Face))
	case !errors.Is(getErr, store.ErrNotFound):
		return nil, fmt.Errorf("entitymanager: RecreateEntity: existence check for %s: %w",
			entity.FormatStateRef(e.ID, e.Face), getErr)
	}

	// The same face rule the other create paths enforce
	// (Manager.CreateEntity, ValidateCreate, cascadeHost.CreateEntity), so a
	// body cannot land a row at an undeclared face, or at the zero coordinate
	// of a faced type — a row belonging to no declared face, which nothing can
	// address and no grant covers.
	//
	// Only when the type is DECLARED: an unknown type is validation's to
	// report, and preempting it here would swap a ValidationError for a bare
	// "unknown entity type" that callers cannot classify.
	if _, known := m.deps.Meta.GetEntityDef(e.Type); known {
		if err := m.deps.requireCreateFaceFor(e.Type, e.Face); err != nil {
			return nil, err
		}
	}
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpCreate,
		Subject: acl.NewEntitySubject(e.Type, e.ID, e.Face),
	}); err != nil {
		return nil, err
	}
	// A whole-record write: ignore any incoming materialized value and
	// recompute under the current schema.
	if err := m.deps.Computed.Evaluate(ctx, e); err != nil {
		return nil, err
	}

	// DEC-HWZHA: hard structural errors abort (the API layer maps these to
	// 422); soft conditions surface as warnings on the result.
	hard, soft := partitionValidationErrors(m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties))
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}

	// The state machines' ENTRY rule is deliberately not applied
	// (BUG-KK1UXH): it governs a new record, and a restore brings back one
	// that existed, so a deleted `done` ticket comes back `done`. The
	// transition guards still bind: some path of declared edges from the
	// entry value must reach the restored value with every guard on it held,
	// or delete-then-restore would reach a state a guard keeps the principal
	// out of. `when:` preconditions are not evaluated, because a restore has
	// no prior state for them to judge. See [statemachine.Set.EnforceRestore].
	// RecreateEntity runs no automation, so this is the final pre-write state.
	//
	// The exemption is narrow because this function is: only history restore
	// reaches it (the dataentry handler and `rela restore`), and
	// internal/archguard/recreate_test.go pins its callers. Every ordinary
	// create keeps EnforceCreate in createCore.
	if err := m.deps.Transitions.EnforceRestore(ctx, e, m.deps.TransitionGuard); err != nil {
		return nil, m.mapTransitionError(ctx, acl.OpCreate, acl.NewEntitySubject(e.Type, e.ID, e.Face), err)
	}

	// `unique: true` natural keys are enforced atomically with the write.
	if err := writeWithUniqueCheck(ctx, m.deps, e, e.ID, func(st store.Store) error {
		return persistRecreate(ctx, st, e)
	}); err != nil {
		return nil, err
	}
	m.recordEntityAudit(ctx, audit.OpCreateEntity, e, "created")

	return &entity.UpdateResult{Entity: e, Warnings: soft}, nil
}

// persistRecreate creates e. A conflict is ErrEntityAlreadyExists and never
// falls back to an update, so a recreate that races a concurrent create can
// never become a blind, re-typing overwrite.
func persistRecreate(ctx context.Context, st store.Store, e *entity.Entity) error {
	if err := st.CreateEntity(ctx, e); err != nil {
		// A derived unique-property index rejects a duplicate PROPERTY value
		// (TKT-3Q0GP1). Check before ErrConflict (which UniquePropertyError
		// also satisfies) so it surfaces as the property 422, not the
		// ID-collision error.
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return mapped
		}
		if errors.Is(err, store.ErrConflict) {
			return fmt.Errorf("%w: %s", ErrEntityAlreadyExists, entity.FormatStateRef(e.ID, e.Face))
		}
		return fmt.Errorf("entitymanager: RecreateEntity: %w", err)
	}
	return nil
}

// Recreator adapts [RecreateEntity] to a method-shaped dependency.
type Recreator struct{ M *Manager }

// RecreateEntity calls [RecreateEntity] on the wrapped manager.
func (r Recreator) RecreateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error) {
	return RecreateEntity(ctx, r.M, e)
}
