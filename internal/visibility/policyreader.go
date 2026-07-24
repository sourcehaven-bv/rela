package visibility

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// PolicyReader is the policy-enforcing [Reader]: row-gate first, then
// field-redact a copy. Semantics are hoisted from dataentry's
// visibleReader/copyVisibleProperties (TKT-N26KLB) with one deliberate
// strengthening: the stored-type check lives here, in the package, never
// in consumers (RR-SRZK6X).
type PolicyReader struct {
	gate   RowGate
	redact FieldRedactor
	get    EntityGetter
}

// NewPolicyReader builds a PolicyReader. All collaborators are required
// (constructors-reject-nil rule).
func NewPolicyReader(gate RowGate, redact FieldRedactor, get EntityGetter) (*PolicyReader, error) {
	if gate == nil {
		return nil, errors.New("visibility: NewPolicyReader: gate must be non-nil")
	}
	if redact == nil {
		return nil, errors.New("visibility: NewPolicyReader: redact must be non-nil")
	}
	if get == nil {
		return nil, errors.New("visibility: NewPolicyReader: get must be non-nil")
	}
	return &PolicyReader{gate: gate, redact: redact, get: get}, nil
}

// Get implements [Reader]. Gate BEFORE load (hidden == missing, RR-NGMI),
// then verify the stored type matches the caller's claim (RR-SRZK6X),
// then redact a copy.
func (r *PolicyReader) Get(ctx context.Context, entityType, id string) (*entity.Entity, bool, error) {
	ok, err := r.gate.PermitsRead(ctx, entityType, id)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	e, gerr := r.get.GetEntity(ctx, id)
	if gerr != nil {
		// Store miss == not-found; indistinguishable from a deny by design.
		return nil, false, nil //nolint:nilerr // store miss == not-found, by design
	}
	if e.Type != entityType {
		// The gate authorized the CLAIMED type; acting on a different
		// stored type would be the BUG-ZWTDH9 escalation. Same
		// indistinguishable miss.
		return nil, false, nil
	}
	return r.redacted(ctx, e), true, nil
}

// Filter implements [Reader]: batched row-gate per type (one
// PermitsReadMany per distinct type, RR-FRK1 shape), fail-closed
// type-drop on gate error, then redaction of every survivor. Order is
// preserved and a fresh slice returned; nil for empty input.
func (r *PolicyReader) Filter(ctx context.Context, candidates []*entity.Entity) []*entity.Entity {
	if len(candidates) == 0 {
		return nil
	}
	byType := make(map[string][]string)
	for _, c := range candidates {
		if c == nil {
			continue // fail-closed: a nil candidate must not panic the filter
		}
		byType[c.Type] = append(byType[c.Type], c.ID)
	}
	allowed := r.permittedIDs(ctx, byType)

	out := make([]*entity.Entity, 0, len(candidates))
	for _, c := range candidates {
		if c != nil && allowed[c.ID] {
			out = append(out, r.redacted(ctx, c))
		}
	}
	return out
}

// FilterRelations implements [Reader]: a relation survives only when BOTH
// endpoints are visible (FROM ∧ TO). Endpoint types are resolved with one
// load per distinct endpoint id; visibility with one PermitsReadMany per
// distinct type. A missing endpoint or a gate error hides fail-closed.
func (r *PolicyReader) FilterRelations(ctx context.Context, rels []*entity.Relation) []*entity.Relation {
	if len(rels) == 0 {
		return nil
	}
	byType := make(map[string][]string)
	seen := make(map[string]bool)
	for _, rel := range rels {
		if rel == nil {
			continue // fail-closed: a nil relation must not panic the filter
		}
		for _, id := range [2]string{rel.From, rel.To} {
			if seen[id] {
				continue
			}
			seen[id] = true
			e, err := r.get.GetEntity(ctx, id)
			if err != nil {
				continue // missing endpoint: stays out of allowed → relation hidden
			}
			byType[e.Type] = append(byType[e.Type], id)
		}
	}
	allowed := r.permittedIDs(ctx, byType)

	out := make([]*entity.Relation, 0, len(rels))
	for _, rel := range rels {
		if rel != nil && allowed[rel.From] && allowed[rel.To] {
			out = append(out, rel)
		}
	}
	return out
}

// permittedIDs runs one PermitsReadMany per distinct type and returns the
// union allowed-id set. A gate error drops that whole type fail-closed —
// a read-ACL failure must never widen visibility — and is logged loud so
// operators see the cause rather than silently thinner results.
func (r *PolicyReader) permittedIDs(ctx context.Context, byType map[string][]string) map[string]bool {
	allowed := make(map[string]bool)
	for typeName, ids := range byType {
		perm, err := r.gate.PermitsReadMany(ctx, typeName, ids)
		if err != nil {
			slog.Warn("visibility: PermitsReadMany failed; dropping type fail-closed",
				"type", typeName, "candidates", len(ids), "err", err)
			continue
		}
		for id, ok := range perm {
			if ok {
				allowed[id] = true
			}
		}
	}
	return allowed
}

// redacted returns e with hidden properties stripped. When nothing is
// hidden the ORIGINAL pointer is returned (read-out contract: callers
// must not mutate) — this keeps the no-policy path allocation-free and
// byte-identical to a raw read. When redaction applies, the struct is
// shallow-copied with a fresh filtered Properties map; property VALUES
// still alias the originals, which is safe because read-out paths
// serialize before anything can alias (the copyVisibleProperties
// contract, TKT-IHC7D).
//
// No separate title fallback is needed here: unlike the wire DTO (whose
// precomputed _title stripHiddenProperties must rewrite), an
// entity.Entity carries no secondary title channel — DisplayTitle
// derivations recompute from Properties and fall back to the ID once a
// hidden display property is stripped.
//
// TODO(body-redaction): Content and Inaccessible pass through verbatim.
// Today that is correct — the `visible:` policy universe is
// metamodel-declared properties, so a body can't be policy-hidden — but
// if body-level redaction ever becomes policy-expressible
// (entity.InaccessibleFieldContent exists as the reserved marker), this
// is the spot that must learn about it, or the seam silently leaks
// bodies (RR-J6022V).
func (r *PolicyReader) redacted(ctx context.Context, e *entity.Entity) *entity.Entity {
	hidden := r.redact.HiddenProperties(ctx, e)
	if len(hidden) == 0 {
		return e
	}
	out := *e
	out.Properties = filterProps(e.Properties, hidden)
	return &out
}

// filterProps returns a fresh map of props minus hidden. Never mutates
// props (which may alias live store state).
func filterProps(props map[string]any, hidden map[string]struct{}) map[string]any {
	out := make(map[string]any, len(props))
	for k, v := range props {
		if _, h := hidden[k]; h {
			continue
		}
		out[k] = v
	}
	return out
}
