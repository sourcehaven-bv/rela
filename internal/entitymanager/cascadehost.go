package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// cascadeHost satisfies [autocascade.Host]. It is the surface
// [autocascade.Runner] calls back into during a cascade. cascadeHost
// is constructed per-call inside [Manager.CreateEntity] /
// [Manager.UpdateEntity] (never held as a field) so its lifetime is
// scoped to a single Process call — the form CLAUDE.md's
// "consumer-side interfaces for callbacks" pattern endorses for
// dissolving cycles.
//
// **Important contract:** [cascadeHost.CreateEntity] must NOT fire
// follow-up automation cascades. Runner is the one that schedules
// cascade evaluation on the returned entity; double-cascading would
// enforce [autocascade.MaxDepth] twice and reorder creations.
//
// Audit: cascadeHost emits audit records directly (bypassing
// Manager's recordEntityAudit / recordRelationAudit) because it
// bypasses Manager itself — going through createCore / direct store
// writes to avoid double-cascading. Records carry a triggered_by label
// distinguishing them from direct writes: normally
// `automation:<name>`, stamped on the ctx by the autocascade runner
// (TKT-JJRVX9); `cascade:delete-entity:<id>` for the relation deletes
// under an IfExistsReplace; and a bare `automation` only when the
// automation was declared without a name.
type cascadeHost struct {
	deps Deps
}

// Compile-time assertion.
var _ autocascade.Host = (*cascadeHost)(nil)

// CreateEntity satisfies [autocascade.Host.CreateEntity]. It calls
// the package-level [createCore] helper directly, **without**
// running automations afterward (Runner manages follow-up cascade
// scheduling on the result).
func (h *cascadeHost) CreateEntity(
	ctx context.Context, entityType string, opts autocascade.CreateEntityOptions,
) (*entity.Entity, error) {
	// Cascade-driven creates discard warnings — the autocascade.Host
	// contract returns only (*entity.Entity, error). The Runner doesn't
	// propagate per-step warnings; they'd be merged into the trigger's
	// entity.CreateResult.Warnings if we extended Outcome, but that's a
	// separate change.
	// A cascade cannot name a face: autocascade.CreateEntityOptions carries
	// none, and the automation DSL has no syntax for one. Refusing is the
	// fail-closed answer for a faced type — writing the zero coordinate would
	// mint a row belonging to NO declared face, which nothing can address and
	// no grant covers (BUG-HC6I2T). requireCreateFace states the same rule the
	// ordinary create path enforces, so the two cannot drift.
	if err := h.deps.requireCreateFaceFor(entityType, ""); err != nil {
		return nil, err
	}
	e, _, err := createCore(ctx, h.deps, entityType, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.IDPrefix,
		TemplateVariant: opts.TemplateVariant,
		Properties:      opts.Properties,
		Content:         opts.Content,
	})
	if err == nil {
		h.recordCascade(ctx, audit.OpCreateEntity, entitySubject(e), "created")
	}
	return e, err
}

// WriteEntity satisfies [autocascade.Host.WriteEntity] by applying set to an
// already-persisted entity.
//
// It is an UPDATE, never an upsert: the Runner only calls WriteEntity to
// persist post-cascade property changes onto an entity it created via
// CreateEntity earlier in the SAME cascade, so the row is guaranteed to
// exist. A create-then-update fallback here would be the lost-update /
// type-re-type vector removed in BUG-ZWTDH9.
//
// It is a compare-and-swap onto the stored row (see
// [writeAutomationProperties]), so a patch that landed after the cascade's
// create is kept.
//
// Note: no audit record here. The earlier CreateEntity already produced
// one audit record for this entity; emitting another for the property-set
// step would double-count the same creation in the audit log.
//
// Constraints are RE-CHECKED against the post-automation values (BUG-KIMZRK).
// createCore validated the candidate as it stood BEFORE automation ran, so a
// value an automation introduces afterwards has never been examined. The
// top-level create path already re-runs the same checks for exactly this
// reason ("the create path must not be the weaker one" — see CreateEntity);
// without them here, an automation could silently persist a duplicate of a
// `unique:` natural key, or a value validation would reject.
func (h *cascadeHost) WriteEntity(ctx context.Context, e *entity.Entity, set map[string]string) error {
	if e == nil { // coverage-ignore: defensive: Runner only calls WriteEntity with an entity it created earlier in the
		// same cascade; never nil
		return nil
	}
	// writeAutomationProperties rejects a computed property in set, so the
	// check needs no computed comparison: next's computed values were just
	// re-evaluated and may legitimately differ from stored's.
	_, err := writeAutomationProperties(ctx, h.deps, e, set, func(_, next *entity.Entity) error {
		if errs := h.deps.Meta.ValidateEntity(next.ID, next.Type, next.Properties); len(errs) > 0 {
			// DEC-HWZHA: only HARD errors abort. Soft conditions (a required
			// property left unset, an out-of-enum value) are tolerated on every
			// other write path and must stay tolerated here, or a cascade would
			// be stricter than a direct edit.
			if hard, _ := partitionValidationErrors(errs); len(hard) > 0 {
				return newValidationError(hard)
			}
		}
		// EnforceCreate, not EnforceUpdate: this row was created moments ago
		// in this same cascade, so the automation's value is still an ENTRY
		// value — it must equal the machine's declared entry, not merely be
		// reachable from it by a legal move. Using EnforceUpdate would wrongly
		// accept a one-hop jump the create path forbids.
		return h.deps.Transitions.EnforceCreate(ctx, next)
	})
	return err
}

// EntityType satisfies [autocascade.Host.EntityType] by reading the family
// of id, so a faced target is found (BUG-J3PBFN).
func (h *cascadeHost) EntityType(ctx context.Context, id string) (string, error) {
	fam, err := lookupFamily(ctx, h.deps.Store, id)
	if err != nil {
		return "", err
	}
	return fam.typ, nil
}

// WriteRelation satisfies [autocascade.Host.WriteRelation] by CREATING
// the automation-generated relation.
//
// It is a create, never an upsert: the Runner only calls WriteRelation
// for freshly built [automation.Result.RelationsToCreate] and trigger
// relations, so the intent is always create. A store.ErrConflict means
// the identical triple already exists — an idempotent re-trigger of the
// same automation, or a concurrent writer that created the same triple
// first. Either way the relation exists, so treat it as a no-op success
// rather than blindly overwriting it (which was the removed create-then-update fallback,
// BUG-ZWTDH9); no audit record is emitted for the no-op since nothing
// was written.
func (h *cascadeHost) WriteRelation(ctx context.Context, r *entity.Relation) error {
	if r == nil { // coverage-ignore: defensive: Runner only calls WriteRelation with freshly built
		// RelationsToCreate/trigger relations; never nil
		return nil
	}
	// The Runner hands over the trigger row's face. A content-scoped edge
	// belongs to that face; an identity-scoped one to the entity, whose only
	// valid tail is the zero face (see requireRelationFaceFor).
	r.FromFace = h.deps.cascadeTail(r.Type, r.FromFace)
	// This path writes the store directly, so it applies the owning rules
	// itself (TKT-QO14GB), in one Tx with the write. Re-creating the
	// identical owning edge passes the check and is the idempotent no-op
	// below.
	owning := metamodel.IsOwning(h.deps.Meta, r.Type)
	write := func(st store.Store) error {
		if err := CheckOwningEdge(ctx, h.deps.Meta, st, r.Identity()); err != nil {
			return err
		}
		_, err := st.CreateRelation(ctx, r.Identity(), &store.RelationData{
			Properties: r.Properties,
			Content:    r.Content,
		})
		return err
	}
	var err error
	if owning {
		err = h.deps.Store.Tx(ctx, write)
	} else {
		err = write(h.deps.Store)
	}
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil
		}
		return err
	}
	h.recordCascade(ctx, audit.OpCreateRelation, relationSubject(r), "created")
	return nil
}

// ValidateRelation satisfies [autocascade.Host.ValidateRelation] by
// delegating to the metamodel.
func (h *cascadeHost) ValidateRelation(relType, fromType, toType string) error {
	return h.deps.Meta.ValidateRelation(relType, fromType, toType)
}

// DeleteEntity satisfies [autocascade.Host.DeleteEntity]. The entityType
// parameter is informational.
//
// It delegates to [Manager.DeleteEntity], the family path, so an
// if_exists: replace delete removes every face of a faced entity and records
// one audit row and one version per face, plus a version for each cascaded
// relation (BUG-J3PBFN). The reimplementation this replaces was face-blind:
// it wrote one audit record and captured no version.
//
// The handle is a cascade writer: the automation that triggered the delete
// authorized it, as it does every other cascadeHost write. The records carry
// the `automation` label when the ctx names no automation, as recordCascade
// stamps it, so an unnamed automation's delete does not read as the user's.
func (h *cascadeHost) DeleteEntity(ctx context.Context, _, id string, cascade bool) error {
	if audit.TriggeredByFrom(ctx) == "" {
		ctx = audit.WithTriggeredBy(ctx, "automation")
	}
	m := &Manager{deps: h.deps, cascadeWrite: true}
	if _, err := m.DeleteEntity(ctx, id, cascade); err != nil {
		return fmt.Errorf("delete entity: %w", err)
	}
	return nil
}

// FindExistingRelationTarget satisfies
// [autocascade.Host.FindExistingRelationTarget].
func (h *cascadeHost) FindExistingRelationTarget(
	ctx context.Context, source entity.Ref, relationType, targetType string,
) *entity.Entity {
	return findExistingRelationTarget(ctx, h.deps, source, relationType, targetType)
}

// entitySubject builds the Subject for an entity-shaped audit record.
func entitySubject(e *entity.Entity) *audit.Subject {
	return &audit.Subject{
		Kind: "entity",
		Type: e.Type,
		ID:   e.ID,
	}
}

// relationSubject builds the Subject for a relation-shaped audit record.
func relationSubject(r *entity.Relation) *audit.Subject {
	return &audit.Subject{
		Kind:         "relation",
		RelationType: r.Type,
		FromID:       r.From,
		ToID:         r.To,
	}
}

// recordCascade emits one audit record from the cascade path.
//
// The ctx label wins when there is one: the autocascade runner stamps
// `automation:<name>` per entry, and a caller may have stamped something more
// specific still (`cascade:delete-entity:<id>`). "automation" is only the
// fallback for a cascade whose originating automation had no name — since
// TKT-JJRVX9 it is no longer the normal case, and this `if` is now what makes
// the runner's per-entry label survive rather than what supplies attribution.
//
// The same `if` is mirrored one layer up in autocascade's triggeredByCtx,
// which likewise declines to overwrite an existing label — that is what keeps
// a scheduler's `schedule:<task>` on rows an automation cascaded underneath it.
//
// One emitter for both entity and relation subjects — the Subject
// face carries the shape; the rest of the Record envelope is
// identical.
func (h *cascadeHost) recordCascade(
	ctx context.Context, op string, subject *audit.Subject, summary string,
) {
	if subject == nil { // coverage-ignore: defensive: all callers pass entitySubject()/relationSubject() results, which
		// are never nil
		return
	}
	if audit.TriggeredByFrom(ctx) == "" {
		ctx = audit.WithTriggeredBy(ctx, "automation")
	}
	h.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}
