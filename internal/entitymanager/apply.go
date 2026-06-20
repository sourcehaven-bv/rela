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

// upsertOp captures the create-vs-update labels for an apply, derived once from
// whether the target already exists.
type upsertOp struct {
	aclOp   acl.Op
	auditOp string
	summary string
}

// resolveUpsertOp turns the result of an existence probe into the create-or-
// update labels for an upsert.
//
// It fails CLOSED: a non-nil error that is NOT [store.ErrNotFound] (a flaky
// backend, a cancelled context) is returned as an error rather than being
// treated as "does not exist". Treating a transient read failure as a create
// would (a) authorize the wrong ACL verb — create and update are separately
// grantable — and (b) write a "create" audit row for what is really an update,
// corrupting the forensic log during exactly the incident you'd be
// investigating. This mirrors the fail-closed discipline in [Manager.RenameEntity].
func resolveUpsertOp(getErr error, createAudit, updateAudit string) (upsertOp, error) {
	switch {
	case getErr == nil:
		return upsertOp{aclOp: acl.OpUpdate, auditOp: updateAudit, summary: "updated"}, nil
	case errors.Is(getErr, store.ErrNotFound):
		return upsertOp{aclOp: acl.OpCreate, auditOp: createAudit, summary: "created"}, nil
	default:
		return upsertOp{}, getErr
	}
}

// ApplyEntity upserts an entity by its supplied ID, preserving that ID, with
// the full ACL + validation + audit framing of the normal write path but
// WITHOUT running automation or the cascade.
//
// It exists for the sync apply path (FEAT-NJ9FEN): a record edited on one peer
// must be reproduced on the other under its existing ID, and the automations
// that produced any derived records already ran on the origin — re-running them
// here would duplicate side effects and can ping-pong derived changes back
// (TKT-78R2YB; design review RR-L1MY0N, RR-AZMA7T).
//
// How it differs from the human-intent write path:
//
//   - Unlike [Manager.CreateEntity], it does NOT reject an explicit ID for a
//     non-manual id_type, generate an ID, or apply a template / status default.
//     The caller owns the complete entity state (id, type, properties,
//     content); ApplyEntity persists exactly that. PRECONDITION: the caller
//     must supply every field the record should have (including status) — there
//     is no backfill, because automation is suppressed.
//   - Unlike CreateEntity/UpdateEntity, it runs NO automation and NO cascade.
//   - Like every write path, it authorizes against the ACL, validates against
//     the metamodel (hard errors abort; soft conditions ride along as
//     warnings), and emits an audit record AFTER the durable write (consistent
//     with Create/Update — a committed write is never left unaudited). It must
//     not be confused with the internal upsertEntity, a raw store write with
//     none of that.
//
// ID-prefix note: validation includes the metamodel's ID-prefix check, which is
// a HARD error. Sync therefore assumes peers share a metamodel (so a
// peer-minted ID matches a local prefix). A record whose ID matches no local
// prefix is rejected — see TestApplyEntity_RejectsForeignIDPrefix.
//
// The audited op (create vs. update) is chosen from whether the entity already
// exists; a flaky existence probe fails closed (see [resolveUpsertOp]).
func (m *Manager) ApplyEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error) {
	if e == nil {
		return nil, errors.New("entitymanager: ApplyEntity: entity is nil")
	}
	if e.ID == "" {
		return nil, errors.New("entitymanager: ApplyEntity: entity ID is empty")
	}
	if e.IsLocked() {
		// The in-memory entity has redacted fields; writing it would replace
		// the on-disk content (typically ciphertext) with the cleartext shell.
		// Same guard the rest of the write path applies.
		return nil, fmt.Errorf("entitymanager: ApplyEntity: entity %s has inaccessible fields", e.ID)
	}

	_, getErr := m.deps.Store.GetEntity(ctx, e.ID)
	op, err := resolveUpsertOp(getErr, audit.OpCreateEntity, audit.OpUpdateEntity)
	if err != nil {
		return nil, fmt.Errorf("entitymanager: ApplyEntity: existence check for %s: %w", e.ID, err)
	}

	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      op.aclOp,
		Subject: acl.EntitySubject{Type: e.Type, ID: e.ID},
	}); err != nil {
		return nil, err
	}

	// DEC-HWZHA: hard structural errors abort (the API layer maps these to
	// 422); soft conditions surface as warnings on the result.
	hard, soft := partitionValidationErrors(m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties))
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}

	if err := upsertEntity(ctx, m.deps.Store, e); err != nil {
		return nil, fmt.Errorf("entitymanager: ApplyEntity: %w", err)
	}
	m.recordEntityAudit(ctx, op.auditOp, e, op.summary)

	return &entity.UpdateResult{Entity: e, Warnings: soft}, nil
}

// ApplyRelation upserts a relation by its from/type/to triple, with the same
// ACL + validation + audit framing as the human-intent relation write path but
// WITHOUT managed-order auto-assignment — the caller's relation already carries
// any order properties from the origin.
//
// Both endpoints must already exist (a relation cannot reference a missing
// entity), so the sync apply layer is responsible for ordering: all entities
// before any relation that references them (design review RR-YHGJHG). A genuine
// missing endpoint returns [ErrEntityNotFound] (the apply layer retries on the
// next pass once the endpoint is applied); a flaky endpoint read fails closed
// with the underlying error, so the retry loop is not spun forever on a
// transient backend fault.
//
// Managed order: unlike CreateRelation/UpdateRelation, ApplyRelation does not
// auto-assign or validate _order_out/_order_in — sync mirrors whatever finite
// order the origin already assigned. The caller (sync) is trusted to carry
// well-formed order values from a peer that produced them through the normal
// write path.
func (m *Manager) ApplyRelation(ctx context.Context, r *entity.Relation) (*entity.Relation, error) {
	if r == nil {
		return nil, errors.New("entitymanager: ApplyRelation: relation is nil")
	}
	if r.IsLocked() {
		return nil, fmt.Errorf("entitymanager: ApplyRelation: relation %s has inaccessible fields", r.Key())
	}

	fromEntity, err := m.requireEndpoint(ctx, r.From, "source")
	if err != nil {
		return nil, err
	}
	toEntity, err := m.requireEndpoint(ctx, r.To, "target")
	if err != nil {
		return nil, err
	}

	_, getErr := m.deps.Store.GetRelation(ctx, r.From, r.Type, r.To)
	op, err := resolveUpsertOp(getErr, audit.OpCreateRelation, audit.OpUpdateRelation)
	if err != nil {
		return nil, fmt.Errorf("entitymanager: ApplyRelation: existence check for %s: %w", r.Key(), err)
	}

	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: op.aclOp,
		Subject: acl.RelationSubject{
			Type:     r.Type,
			FromType: fromEntity.Type, FromID: r.From,
		},
	}); aclErr != nil {
		return nil, aclErr
	}

	if vErr := m.deps.Meta.ValidateRelation(r.Type, fromEntity.Type, toEntity.Type); vErr != nil {
		return nil, fmt.Errorf("entitymanager: ApplyRelation: invalid relation: %w", vErr)
	}

	if err := upsertRelation(ctx, m.deps.Store, r); err != nil {
		return nil, fmt.Errorf("entitymanager: ApplyRelation: %w", err)
	}
	m.recordRelationAudit(ctx, op.auditOp, r, op.summary)

	return r, nil
}

// requireEndpoint loads a relation endpoint, distinguishing a genuine
// not-found (mapped to [ErrEntityNotFound] so the sync layer retries) from a
// transient store error (returned as-is so the retry loop is not spun forever).
// role is "source" or "target" for the error message.
func (m *Manager) requireEndpoint(ctx context.Context, id, role string) (*entity.Entity, error) {
	ent, err := m.deps.Store.GetEntity(ctx, id)
	switch {
	case err == nil:
		return ent, nil
	case errors.Is(err, store.ErrNotFound):
		return nil, fmt.Errorf("%s %w: %s", role, ErrEntityNotFound, id)
	default:
		return nil, fmt.Errorf("entitymanager: ApplyRelation: load %s endpoint %s: %w", role, id, err)
	}
}
