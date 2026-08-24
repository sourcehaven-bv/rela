package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// copyEngine owns the copy operation: planning, authorization, target
// construction, edge planning and audit.
//
// # Why a separate type (Jeroen's call, TKT-WRLDAPI item 5)
//
// It is a decomposition, not a lint workaround. Copies are already a distinct
// operation — deliberately NOT on the [EntityManager] interface, which is the
// ordinary-CRUD write contract — and this makes that boundary structural
// rather than conventional. `Manager` was at 41 methods against a 40 load
// line, almost all of them private helpers; adding the copy-affordance query
// would have pinned it at the line rather than under it, and the load line is
// a ratchet to narrow, not a budget to spend.
//
// # It holds the Manager, and that is deliberate
//
// The engine needs `authorizeAndAudit`, which is Manager's and is shared with
// every other write path. Duplicating it here would be a second authorization
// implementation — the exact defect this feature's design refuses. So the
// engine composes the manager rather than replacing it.
//
// # planCopy and authorizeCopy stay UNEXPORTED
//
// That is load-bearing, not incidental. [CopiesForSource] computes each
// offer's `Allowed` by running those two — the same code the invoke runs — and
// that is the only reason the hint cannot drift from the write (RULING 11).
// Exporting them, or making invocability computable from outside this package,
// would let a caller answer the question a different way and reintroduce the
// two-authorization-sites defect.
type copyEngine struct{ m *Manager }

// Copy errors. ErrUnknownCopy is caller input (4xx); the guard errors map to
// 403/422 exactly as the statemachine's do, because they ARE the
// statemachine's vocabulary (design doc §9.1: shared guard machinery,
// separate declaration).
var (
	// ErrUnknownCopy names a definition the metamodel does not declare. A
	// request may only invoke a definition BY NAME, so this is the whole of
	// "that copy does not exist" — never a hint that some other mapping
	// might work.
	ErrUnknownCopy = errors.New("entitymanager: no such copy definition")

	// ErrCopySourceMissing means the source face does not exist. Distinct
	// from a denial: the caller asked to copy something that is not there.
	ErrCopySourceMissing = errors.New("entitymanager: copy source face does not exist")
)

// CopyRequest invokes a declared copy definition. There is deliberately no
// field for the mapping: a request chooses a NAME, never a shape.
//
// This is the transforms-registry precedent (`?transform=<name>`, never a
// command), and it is what keeps the guard system meaningful — a caller who
// could describe an arbitrary copy could describe one whose guard is
// convenient.
type CopyRequest struct {
	// Definition is the key in the metamodel's `copies:` block.
	Definition string

	// SourceID is the entity the copy reads from. For a same-entity copy it
	// is also the target; for a cross-entity copy the target is created.
	SourceID string

	// TargetID is the cross-entity target's id. Ignored (and must be empty)
	// for a same-entity copy, whose target is SourceID by construction.
	TargetID string
}

// CopyResult reports what a copy produced, in the vocabulary a UI would
// need without deciding anything about presentation.
type CopyResult struct {
	// Definition is the name that was invoked.
	Definition string
	// Entity is the resulting target face, as stored.
	Entity *entity.Entity
	// Created is true when the copy brought the target face into existence
	// rather than overwriting one.
	Created bool
}

// CopyState executes a declared copy definition: write a mapped subset of one
// content state (face) into another, in ONE store transaction, audited.
//
// # The kernel is dumb; the definition carries the policy
//
// Everything policy-shaped — which fields, merge vs replace, the guard —
// lives in the compiled definition. This function writes what it is told.
// That split is what lets the security tests aim at one small function
// rather than at a mapping language.
//
// # store.Tx, and a note for reviewers
//
// entitymanager had NEVER called store.Tx before this (the only non-store
// caller in the tree was internal/datamigration). That is deliberate here,
// not an oversight: a copy writes fields, body and edges, and a half-applied
// copy is a face that claims to be a promotion of something it is not.
//
// Two consequences the contract forces:
//
//   - Every store call inside the transaction goes through the VIEW, never
//     ce.m.deps.Store. A write on the outer store from inside deadlocks on
//     fs/mem and silently bypasses the transaction on pg.
//   - fs/mem have NO ROLLBACK — store.Tx there is a write mutex. A mid-copy
//     failure leaves a partially written target face on those backends. That
//     is the accepted best-effort stance for fsstore (§6.1); pgstore gives
//     real atomicity.
//
// # Audit lands AFTER the commit
//
// A rollback that left an audit record would make the log claim a write that
// never happened, which is worse than a missing record — the log's whole
// value is that it does not lie. This matches the existing "audit the
// durable write" rule, extended past a transaction boundary.
//
// # What this does NOT do
//
// It does not validate the DESTINATION WORLD. §9.2 says a copy into a state
// a world selects is valid iff that world remains a valid graph, but
// `must_hold_in` is TKT-9KZGJO's declared seam and shipping half of it here
// would fork it. That is a boundary, not an omission.
func (ce *copyEngine) copyState(ctx context.Context, req CopyRequest) (*CopyResult, error) {
	ctx = withStoreAttribution(ctx)

	def, ok := ce.m.deps.Meta.Copies[req.Definition]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCopy, req.Definition)
	}
	plan, err := ce.planCopy(ctx, req, def)
	if err != nil {
		return nil, err
	}

	tx, ok := ce.m.deps.Store.(store.Transactor)
	if !ok {
		// Every backend implements Transactor; a store that does not is a
		// wiring error, and running the copy unsynchronized would be worse
		// than refusing it.
		return nil, errors.New("entitymanager: store does not support transactions")
	}

	var result CopyResult
	if err := tx.Tx(ctx, func(view store.Store) error {
		res, cerr := applyCopy(ctx, view, plan)
		if cerr != nil {
			return cerr
		}
		result = *res
		return nil
	}); err != nil {
		return nil, err
	}

	// AFTER the commit — see the godoc.
	ce.recordCopyAudit(ctx, plan, result)
	return &result, nil
}

// copyPlan is a resolved, authorized copy: the exact bytes to write and where.
// Producing it is where every read, guard and authorization happens, so the
// transaction body stays a pure write.
type copyPlan struct {
	name     string
	def      metamodel.CopyDef
	from     metamodel.CopyTarget
	to       metamodel.CopyTarget
	sourceID string
	targetID string

	// sourceTail and targetTail are the STORED coordinates, resolved once
	// through metamodel.StoredPointer. Resolving them here rather than at
	// each use is not tidiness: a declared pointer marked `default: true` IS
	// the zero coordinate, so a site that used the declared name would write
	// a face at a tail no face lives at — silently, with no error.
	sourceTail entity.Pointer
	targetTail entity.Pointer

	// entity is the target face to write, already merged against any existing
	// target (mapped fields written, unmapped untouched — §9.1).
	entity *entity.Entity
	// created records whether the target face existed before.
	created bool
	// edges are the copied relations, already authorized.
	edges []copyEdge
}

type copyEdge struct {
	relType string
	to      string
	// replace means the target face's edges of this type are removed first.
	replace bool
}

// planCopy resolves the definition against the request: reads the source
// face, evaluates the guard, authorizes the write, and merges the target.
//
// # The elevation split lives here (§9.2)
//
//   - SAME-ENTITY copies read the source RAW. Hidden fields travel with the
//     entity, the principal never sees them, and the same policy governs them
//     on the target face — identity preserved, so policy follows. This is the
//     positive form of the never-redact-a-write-prep rule.
//   - CROSS-ENTITY copies read through the CALLER'S visibility gate, so only
//     fields they may read carry. An elevated cross-entity copy would launder
//     fields the principal cannot read into an entity with a different
//     audience — a redaction bypass, and the reason `fields: all` is a load
//     error on that form.
func (ce *copyEngine) planCopy(
	ctx context.Context, req CopyRequest, def metamodel.CopyDef,
) (*copyPlan, error) {
	from, err := metamodel.ParseCopyTarget(def.From)
	if err != nil {
		return nil, fmt.Errorf("entitymanager: copy %q: from: %w", req.Definition, err)
	}
	to, err := metamodel.ParseCopyTarget(def.To)
	if err != nil {
		return nil, fmt.Errorf("entitymanager: copy %q: to: %w", req.Definition, err)
	}

	plan := &copyPlan{
		name: req.Definition, def: def, from: from, to: to,
		sourceID: req.SourceID, targetID: req.SourceID,
		sourceTail: entity.Pointer(metamodel.StoredPointer(ce.m.deps.Meta, from.Type, from.Pointer)),
		targetTail: entity.Pointer(metamodel.StoredPointer(ce.m.deps.Meta, to.Type, to.Pointer)),
	}
	if !def.IsSameEntity() {
		plan.targetID = req.TargetID
	}

	src, err := ce.readCopySource(ctx, def, plan)
	if err != nil {
		return nil, err
	}
	if err := ce.authorizeCopy(ctx, plan); err != nil {
		return nil, err
	}
	if err := ce.buildCopyTarget(ctx, plan, src); err != nil {
		return nil, err
	}
	return plan, nil
}

// readCopySource performs the elevation split's read half.
func (ce *copyEngine) readCopySource(
	ctx context.Context, def metamodel.CopyDef, plan *copyPlan,
) (*entity.Entity, error) {
	ptr := plan.sourceTail

	if def.IsSameEntity() {
		// RAW and elevated. The read feeds a write, and a redacted read that
		// feeds a write destroys the hidden fields it could not see — the
		// precise bug the never-redact-a-write-prep rule pins.
		e, err := ce.m.deps.Store.GetEntityState(ctx, plan.sourceID, ptr)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrCopySourceMissing, plan.sourceID)
		}
		return e, nil
	}

	// Cross-entity: through the caller's gate. A nil gate means no ACL is
	// wired, which is the CLI/no-policy case — the raw read is then what
	// every other read on that deployment already does.
	if ce.m.deps.CopyVisibility == nil {
		e, err := ce.m.deps.Store.GetEntityState(ctx, plan.sourceID, ptr)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrCopySourceMissing, plan.sourceID)
		}
		return e, nil
	}
	e, ok, err := ce.m.deps.CopyVisibility.Get(ctx, plan.from.Type, plan.sourceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Denied and absent are indistinguishable, by design: whether an
		// entity exists is a genuine secret.
		return nil, fmt.Errorf("%w: %s", ErrCopySourceMissing, plan.sourceID)
	}
	return e, nil
}

// authorizeCopy authorizes the copy.
//
// # Three checks, and why none of them is redundant
//
// §9.2 says a guarded state is writable ONLY by copy definitions naming it as
// target, "each carrying its own guard". That sentence is CONDITIONED on the
// guard existing — it is not a license to skip authorization when a
// definition happens to omit one, which is the hole an earlier cut of this
// function shipped: a principal holding nothing could promote a stranger's
// draft, including fields they could not read.
//
//  1. READ on the source, always. A copy is a read followed by a write, and
//     the elevation split is about which FIELDS travel, not about whether
//     the principal may touch the entity at all. Without this, elevation
//     becomes a way to read what you cannot read.
//  2. The definition's `guard:`, resolved per-entity via
//     HoldsPermissionForEntity, so "the owner of THIS doc may publish it"
//     works without a global grant. Mandatory at LOAD for any definition
//     targeting a non-default face, so the sentence above always applies.
//  3. WRITE on the target — for a CROSS-ENTITY copy only. That creates an
//     entity with its own audience, so it is an ordinary create and must not
//     become a way to make entities the principal could not make by hand.
//
// The same-entity target face is deliberately exempt from (3): nobody holds
// `update` on published by design, so requiring it would make every promote
// impossible. (1) and (2) are what stand in its place.
func (ce *copyEngine) authorizeCopy(ctx context.Context, plan *copyPlan) error {
	// (1) READ on the source. Same-entity copies read RAW afterwards, which
	// is about hidden FIELDS traveling with the entity — it is not a
	// license to read an entity the principal has no access to.
	if ce.m.deps.CopyReadGate != nil {
		permitted, err := ce.m.deps.CopyReadGate.PermitsRead(ctx, plan.from.Type, plan.sourceID)
		if err != nil {
			return err
		}
		if !permitted {
			// Indistinguishable from absent, matching every other read gate.
			return fmt.Errorf("%w: %s", ErrCopySourceMissing, plan.sourceID)
		}
	}

	if perm := plan.def.Guard.Permission; perm != "" {
		if ce.m.deps.CopyGuard == nil {
			// A guarded copy with no guard implementation fails CLOSED,
			// matching the statemachine's nil-guard rule: a guarded edge with
			// no guard is denied, never waved through.
			return &acl.ForbiddenError{Decision: acl.Decision{
				RuleKind: "copy-guard", RuleID: perm,
				Reason: fmt.Sprintf(
					"copy %q requires permission %q but no guard is wired",
					plan.name, perm),
			}}
		}
		if !ce.m.deps.CopyGuard.HoldsPermission(ctx, plan.sourceID, perm) {
			return &acl.ForbiddenError{Decision: acl.Decision{
				RuleKind: "copy-guard", RuleID: perm,
				Reason: fmt.Sprintf(
					"copy %q requires permission %q on %q",
					plan.name, perm, plan.sourceID),
			}}
		}
	}

	if plan.def.IsSameEntity() {
		// (3) does not apply — see above.
		return nil
	}
	return ce.m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpCreate,
		Subject: acl.EntitySubject{
			Type:    plan.to.Type,
			ID:      plan.targetID,
			Pointer: plan.targetTail,
		},
	})
}

// buildCopyTarget merges the source into the target face.
//
// Mapped fields are WRITTEN, unmapped fields are UNTOUCHED on an existing
// target (§9.1's partial-copy semantics); `fields: all` is the full-replace
// promote case, and is only reachable on a same-entity copy because the
// cross-entity form rejects it at load.
func (ce *copyEngine) buildCopyTarget(
	ctx context.Context, plan *copyPlan, src *entity.Entity,
) error {
	targetPtr := plan.targetTail

	existing, err := ce.m.deps.Store.GetEntityState(ctx, plan.targetID, targetPtr)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		plan.created = true
	default:
		// Reading "does the target exist" must not fold a backend failure
		// into "no". A transient read error would otherwise become a
		// CreateEntity over a live face — a silent full overwrite whose audit
		// record cheerfully says created=true.
		return fmt.Errorf("entitymanager: copy %q: probe target face: %w", plan.name, err)
	}
	target := &entity.Entity{
		ID: plan.targetID, Type: plan.to.Type, Pointer: targetPtr,
		Properties: map[string]any{},
	}
	if !plan.created {
		target.Properties = maps.Clone(existing.Properties)
		if target.Properties == nil {
			target.Properties = map[string]any{}
		}
		target.Content = existing.Content
	}

	if plan.def.AllFields {
		target.Properties = maps.Clone(src.Properties)
		if target.Properties == nil {
			target.Properties = map[string]any{}
		}
		target.Content = src.Content
	}
	for field, tmpl := range plan.def.Fields {
		target.Properties[field] = interpolateCopyField(tmpl, src)
	}

	plan.entity = target
	edges, err := ce.planCopyEdges(ctx, plan)
	if err != nil {
		return err
	}
	plan.edges = edges
	return nil
}

// planCopyEdges resolves which edges the definition copies, reading the
// SOURCE face's outgoing relations.
//
// An omitted relation type is NOT copied, and that default is load-bearing:
// copying a role-conferring edge grants roles on the target, so a definition
// must name a type before its edges travel (§9.2's first mitigation).
//
// For a CROSS-ENTITY copy each edge is additionally authorized as the acting
// principal — the mandatory runtime half of that mitigation. A copy can
// never create an edge the principal could not create by hand. Same-entity
// elevated copies only touch the entity's own state edges, so the concern
// does not arise there.
func (ce *copyEngine) planCopyEdges(ctx context.Context, plan *copyPlan) ([]copyEdge, error) {
	if len(plan.def.Relations) == 0 {
		return nil, nil
	}
	srcTail := plan.sourceTail
	crossEntity := !plan.def.IsSameEntity()

	var out []copyEdge
	for rel, err := range ce.m.deps.Store.ListRelations(ctx, store.RelationQuery{
		From: plan.sourceID, FromPointer: &srcTail,
	}) {
		if err != nil {
			return nil, fmt.Errorf("entitymanager: copy %q: read source edges: %w",
				plan.name, err)
		}
		mode, named := plan.def.Relations[rel.Type]
		if !named {
			continue
		}
		if crossEntity {
			if aerr := ce.m.authorizeAndAudit(ctx, acl.WriteRequest{
				Op: acl.OpCreate,
				Subject: acl.RelationSubject{
					Type: rel.Type, FromType: plan.to.Type, FromID: plan.targetID,
				},
			}); aerr != nil {
				return nil, aerr
			}
		}
		out = append(out, copyEdge{
			relType: rel.Type, to: rel.To, replace: mode == "replace",
		})
	}
	return out, nil
}

// recordCopyAudit emits one record per definition invocation, naming the
// definition, source, target and principal — modeled on OpPurgeVersion's
// shape (§9.1), which records WHAT was done to WHICH subject and never the
// content.
//
// It does NOT reuse purge's Manager bypass: purge is a store-level
// destructive op with no entity write, whereas a copy IS an entity write and
// must not have weaker authorization or attribution than a property edit.
func (ce *copyEngine) recordCopyAudit(ctx context.Context, plan *copyPlan, res CopyResult) {
	p := principal.From(ctx)
	ce.m.deps.Audit.Record(audit.Record{
		Op: audit.OpCopyState,
		Subject: &audit.Subject{
			Kind: "entity", Type: plan.to.Type, ID: plan.targetID,
		},
		Principal:   p,
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary: fmt.Sprintf(
			"copy %q: %s -> %s (created=%t)",
			plan.name, copyFaceLabel(plan.sourceID, plan.from),
			copyFaceLabel(plan.targetID, plan.to), res.Created),
	})
}

func copyFaceLabel(id string, t metamodel.CopyTarget) string {
	if t.Pointer == "" {
		return id
	}
	return id + "@" + t.Pointer
}

// CopyState executes a declared copy definition. See [copyEngine.copyState]
// for the full contract; this is the Manager-facing entry point and delegates
// without deciding anything.
func (m *Manager) CopyState(ctx context.Context, req CopyRequest) (*CopyResult, error) {
	return (&copyEngine{m: m}).copyState(ctx, req)
}
