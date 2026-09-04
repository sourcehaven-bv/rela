package visibility

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ReadQueryProvider composes the caller's read scope as a store-level
// predicate instead of a per-row probe. [DeclarativeGate] implements it.
//
// Declared here at the consumer rather than taken as *acl.Declarative so
// ScriptReader binds only to the one call it makes, and so a test can
// supply a stand-in without an ACL policy.
type ReadQueryProvider interface {
	ReadQueryFor(ctx context.Context, entityType string) (acl.ReadQueryResult, error)
}

// rowRedactor is the field-redaction half of a [Reader], reachable without
// the row gate that [Reader.Filter] bundles with it.
//
// Needed because the pushdown replaces the ROW gate but must keep the FIELD
// gate: calling Filter on already-gated rows would re-probe every one of
// them, undoing the amortization the pushdown exists to achieve. Optional
// and type-asserted (the store.Formatter pattern) rather than added to
// Reader, so the four existing implementations stay untouched — and so a
// Reader that does NOT expose it simply forgoes pushdown rather than
// silently skipping redaction.
type rowRedactor interface {
	RedactRow(ctx context.Context, e *entity.Entity) *entity.Entity
}

// RedactRow implements [rowRedactor]: field redaction with no row gate.
// Safe to expose because it NARROWS what a caller sees; it can never widen.
func (r *PolicyReader) RedactRow(ctx context.Context, e *entity.Entity) *entity.Entity {
	return r.redacted(ctx, e)
}

// listPushdown streams the entities of q.Type the caller may READ, using the
// ACL as a store query rather than filtering rows after the fact.
//
// Returns ok=false when pushdown is unavailable (no ReadQueryProvider, or a
// store without GraphQueryer), so the caller falls back to load-then-Filter.
// A fallback is a performance regression, never a correctness or security
// one — both paths gate on the same policy.
//
// WHAT THIS DOES NOT DO: it replaces the ROW GATE only. Field-level
// `visible:` redaction is NOT expressible as a store predicate, so every
// yielded row still goes through the redactor. Dropping that would return
// hidden properties to callers — the #1188 finding this package exists to
// close (RR-1W1G6K). The row gate and the field gate are separate
// mechanisms; pushing one down does not push down the other.
func listPushdown(
	ctx context.Context,
	provider ReadQueryProvider,
	raw store.Store,
	redact func(context.Context, *entity.Entity) *entity.Entity,
	q store.EntityQuery,
) (iter.Seq2[*entity.Entity, error], bool) {
	if provider == nil || q.Type == "" {
		// A type-less list spans every type; the ACL query is composed per
		// type, so there is nothing single to push down.
		return nil, false
	}
	gq, ok := raw.(store.GraphQueryer)
	if !ok {
		return nil, false
	}
	rqr, err := provider.ReadQueryFor(ctx, q.Type)
	if err != nil {
		// Fail CLOSED: a scope we cannot compose must not degrade to an
		// ungated read. Yield the error so the caller surfaces it rather
		// than seeing an empty list that reads as "nothing here".
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }, true
	}

	switch {
	case rqr.DenyAll:
		return func(func(*entity.Entity, error) bool) {}, true
	case rqr.AllowAll:
		// Every row of the type is readable, so the row gate is a no-op --
		// but redaction is NOT (RR-OXE47R). A principal with global read on
		// a type may still be denied individual FIELDS, so "may read every
		// row" must never be shortcut into "may see every property".
		//
		// The FACE filter is not a no-op either, and this branch never builds
		// a GraphQuery — so it rides on the EntityQuery here or the most
		// privileged principals get no face narrowing at all (TKT-O7R2A1).
		// Nil Faces means every face, which is what a `read: ["*"]` wildcard
		// yields, so the common case is unchanged.
		allowQ := q
		allowQ.FaceIn = rqr.Faces
		return redactingSeq(ctx, raw.ListEntities(ctx, allowQ), redact), true
	case rqr.Query == nil:
		// Neither allow, deny, nor query: an unrepresentable state. Treat as
		// a fault and fall back rather than guessing, mirroring
		// acl.PermitsReadMany, which errors here.
		return nil, false
	}
	// Carry the WORLD from the EntityQuery onto the composed GraphQuery
	// (TKT-WAV8XP PR-D). This is the seam the whole two-mechanism design
	// turns on: pushdown reaches PAST every decorator to the raw store,
	// so a world that rides only on the decorator silently degrades to
	// the default world exactly here — and exactly for ACL-gated
	// principals, since AllowAll takes the EntityQuery branch above.
	//
	// That is not hypothetical: it is the fail-open PR-B's review found
	// (RR-GQWRLD), where `otherwise: exclude` stopped excluding and a
	// published world served drafts. `internal/acl` cannot set this
	// itself — arch-lint forbids it importing metamodel, so it cannot
	// compile a WorldScope — which is why the copy happens at this
	// wiring seam instead. Pinned by the decorator/pushdown parity test.
	// COPY the composed query before stamping the world. The ACL layer
	// may cache or reuse the ReadQueryResult per principal, so mutating
	// *rqr.Query in place would leak one request's world into the next
	// caller's — a cross-request scope bleed. The copy is shallow, which
	// is exactly right here: World is a value field, so assigning it
	// touches only this copy, while the predicate faces it shares
	// (HasInbound/HasOutbound/Props) are read-only downstream.
	worldQuery := *rqr.Query
	worldQuery.World = q.World
	// The face allowlist travels with the world, for the same reason and on
	// the same copy: it is computed by the ACL layer but composed here, and
	// mutating rqr.Query in place would leak one principal's face set into
	// the next caller's (TKT-O7R2A1).
	worldQuery.FaceIn = rqr.Faces
	return redactingSeq(ctx, gq.GraphQuery(ctx, worldQuery), redact), true
}

// redactingSeq applies the field redactor to every row of src.
//
// Errors propagate verbatim and terminate the sequence: a store failure must
// reach the caller, never present as a short list (TKT-FVQ4).
func redactingSeq(
	ctx context.Context,
	src iter.Seq2[*entity.Entity, error],
	redact func(context.Context, *entity.Entity) *entity.Entity,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range src {
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(redact(ctx, e), nil) {
				return
			}
		}
	}
}

// ReadQueryFor implements [ReadQueryProvider] by resolving the per-operation
// acl.Request and composing its list-read scope for entityType.
func (g DeclarativeGate) ReadQueryFor(
	ctx context.Context, entityType string,
) (acl.ReadQueryResult, error) {
	r, err := g.request(ctx)
	if err != nil {
		return acl.ReadQueryResult{}, err
	}
	return r.ReadQuery(ctx, entityType), nil
}

// PermittedFaces implements [FaceGate] from the same ReadQueryResult
// [DeclarativeGate.ReadQueryFor] pushes into a list query.
//
// ONE source, two consumers: the list path pushes these faces down into the
// store query, and [PolicyReader] applies them to rows it has already loaded.
// Deriving the set independently in either place is how the two come to
// disagree about which faces a principal may read.
func (g DeclarativeGate) PermittedFaces(
	ctx context.Context, entityType string,
) ([]entity.Face, error) {
	r, err := g.request(ctx)
	if err != nil {
		return nil, err
	}
	return r.ReadQuery(ctx, entityType).Faces, nil
}
