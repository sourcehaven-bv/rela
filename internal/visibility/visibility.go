// Package visibility is the read-side ACL enforcement seam: wrappers that
// row-gate and field-redact entity reads ABOVE the (deliberately ungated)
// base services, per DEC-ZBI39P.
//
// The pattern generalizes [search.VisibleSearcher]: base services stay pure
// and ACL-unaware (store, tracer, search — see the architecture rules in
// CLAUDE.md), and enforcement is structural — a consumer receives a wrapper
// at its wiring site and cannot forget to filter. This replaces the
// gate-by-convention enforcement that let the PR #1188 export paths bypass
// field redaction.
//
// # Contract
//
//   - Gate BEFORE read: a denied row and a nonexistent row are
//     indistinguishable (the RR-NGMI invariant — no existence oracle).
//   - Stored type must equal the caller's claimed type: [Reader.Get]
//     authorizes against the claimed type but verifies the loaded entity's
//     actual type, returning not-found on mismatch (RR-SRZK6X; the
//     read-side analog of BUG-ZWTDH9).
//   - Hidden = nonexistent: trace subtrees below a hidden node are pruned,
//     a path through a hidden intermediate is withheld exactly like
//     no-path, hidden orphans are dropped.
//   - Redaction never mutates stored state: a redacted entity is a copy;
//     the tracer decorator builds fresh property maps and never deletes
//     from the store-aliased ones (RR-6IL3X7).
//   - Fail-closed: a gate or redactor failure hides, never reveals.
//   - Read-out only: these wrappers serve presentation/read-out paths.
//     Write-prep reads (entitymanager diffing) keep raw store access — a
//     redacted read-modify-write would clobber hidden fields on save.
//   - Capability, not identity: a system job that may read everything is
//     handed an [AllowAllReader] at its wiring site while keeping its
//     genuine system principal for audit (the read-side analog of the
//     ElevatedManager pattern, TKT-D8T148). Allow-all is never inferred
//     from the principal.
//
// Accepted residuals (documented, not defended): withholding a computed
// path takes marginally longer than a genuine no-path miss — impractical
// to exploit on in-memory traversal; and [tracer.Tracer.HasCycle] on a
// VISIBLE start still reports a cycle whose loop passes through hidden
// nodes (a single bool of topology).
//
// Under no ACL policy the nop collaborators ([NopGate], [NopRedactor])
// make every wrapper byte-identical to raw access.
package visibility

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RowGate answers entity-level read-permission questions for the principal
// carried on ctx. Consumer-side contract of the acl read gate; the
// production adapter is [DeclarativeGate], the permit-all one is [NopGate].
//
// Neither method verifies existence — they answer "the policy permits
// reading this id IF it exists" (same contract as acl.Request).
type RowGate interface {
	PermitsRead(ctx context.Context, entityType, id string) (bool, error)
	PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error)
}

// FieldRedactor reports the property names hidden from the ctx principal
// for one entity. The production adapter is [PolicyRedactor]; the nop one
// is [NopRedactor].
//
// FAIL-CLOSED CONTRACT (RR-FJUQSF): an implementation that cannot compute
// verdicts must return the hide-everything set (every property name of e),
// never nil — nil means "nothing hidden" and would fail open.
type FieldRedactor interface {
	HiddenProperties(ctx context.Context, e *entity.Entity) map[string]struct{}
}

// EntityGetter is the single-entity load this package needs from the
// store. Satisfied by store.Store.
type EntityGetter interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
}

// Reader is the row-gating, field-redacting entity read-out surface.
// Implementations: [PolicyReader] (policy-enforcing) and [AllowAllReader]
// (explicit pass-through capability for system jobs).
type Reader interface {
	// Get returns the entity when the ctx principal may read it AND its
	// stored type matches entityType. Denied, missing, and type-mismatched
	// are indistinguishable: (nil, false, nil). Only a gate failure is an
	// error — a store-load fault is deliberately swallowed into the same
	// clean miss (the oracle-free contract requires it), so a backend
	// outage reads as 404s; operators debugging phantom misses should
	// check store health, not the gate. The returned entity's PROPERTIES
	// are redacted (hidden names absent). Body redaction is out of scope:
	// the `visible:` policy universe is metamodel-declared properties, so
	// Content is not policy-hideable today and passes through verbatim.
	Get(ctx context.Context, entityType, id string) (*entity.Entity, bool, error)

	// Filter drops candidates the ctx principal may not read and redacts
	// the survivors. Order is preserved; the returned slice is fresh; a
	// gate error drops that whole type fail-closed (logged loud). Nil for
	// empty input.
	Filter(ctx context.Context, candidates []*entity.Entity) []*entity.Entity

	// FilterRelations keeps only relations whose BOTH endpoints are
	// visible to the ctx principal (FROM ∧ TO — the relation-history
	// precedent: the FROM side owns UI placement, it is not the auth
	// boundary). Order preserved, fresh slice, fail-closed on gate error
	// or a missing endpoint. Relations carry no field-level redaction
	// today; row-gating is the whole contract.
	FilterRelations(ctx context.Context, rels []*entity.Relation) []*entity.Relation
}

// HeaderFilterer is [Reader.Filter] for content-free [store.EntityHeader] values
// (TKT-1ESTYJ).
//
// OPTIONAL, kept off [Reader] so a third-party or test Reader need not
// implement it to stay valid. That optionality is safe ONLY because the
// absence of this method degrades to loading whole entities and using
// Filter — strictly more data, never less gating. A Reader that cannot
// filter headers must therefore never be handed headers ungated:
// [ScriptReader.ListEntityHeaders] is the sanctioned entry point, and it
// falls back to whole-entity gating rather than passing rows through.
type HeaderFilterer interface {
	// FilterHeaders drops headers the ctx principal may not read and
	// redacts the survivors, with [Reader.Filter]'s contract: order
	// preserved, fresh slice, fail-closed on gate error, nil for empty
	// input.
	FilterHeaders(ctx context.Context, candidates []store.EntityHeader) []store.EntityHeader
}
