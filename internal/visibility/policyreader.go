package visibility

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PolicyReader is the policy-enforcing [Reader]: row-gate first, then
// field-redact a copy. Semantics are hoisted from dataentry's
// visibleReader/copyVisibleProperties (TKT-N26KLB). Its single-entity read
// is the [Resolver] it holds, built from the same gate and redactor, so the
// list half and the single-entity half cannot disagree about policy.
type PolicyReader struct {
	gate   RowGate
	redact FieldRedactor
	res    *Resolver
}

// NewPolicyReader builds a PolicyReader. All collaborators are required
// (constructors-reject-nil rule); opts configure its [Resolver].
func NewPolicyReader(gate RowGate, redact FieldRedactor, load Loader, opts ...ResolverOption) (*PolicyReader, error) {
	res, err := NewResolver(gate, redact, load, opts...)
	if err != nil {
		return nil, fmt.Errorf("visibility: NewPolicyReader: %w", err)
	}
	return &PolicyReader{gate: gate, redact: redact, res: res}, nil
}

// Resolver returns the single-entity read over this reader's gate, redactor
// and loader.
func (r *PolicyReader) Resolver() *Resolver { return r.res }

// ResolveIDs is [Resolver.ResolveIDs] over this reader's gate and redactor.
func (r *PolicyReader) ResolveIDs(ctx context.Context, w World, ids []string) map[string]store.EntityHeader {
	return r.res.ResolveIDs(ctx, w, ids)
}

// Filter implements [Reader]: batched row-gate per type (one
// ReadableFacesMany per distinct type, RR-FRK1 shape), fail-closed
// type-drop on gate error, then redaction of every survivor. A row survives
// only when its own face passes the verdict. Order is
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
	allowed := r.permittedFaces(ctx, byType)

	out := make([]*entity.Entity, 0, len(candidates))
	for _, c := range candidates {
		if c != nil && allowed[c.ID].Contains(c.Face) && FaceAllowed(ctx, r.gate, c.Type, c.Face) {
			out = append(out, c)
		}
	}
	ctx = PrimeTraversals(ctx, r.redact, out)
	for i, c := range out {
		out[i] = r.redacted(ctx, c)
	}
	return out
}

// FilterHeaders implements [HeaderFilterer]: the [PolicyReader.Filter] contract applied
// to content-free headers.
//
// Identical gating — one ReadableFacesMany per distinct type, order preserved,
// fresh slice, fail-closed on gate error — because it is the SAME policy on
// the same (id, type) pairs. The row gate never consults an entity's body,
// so dropping the body cannot change a verdict.
//
// Field redaction still runs per surviving row: "may read every row of this
// type" is not "may see every property" (RR-OXE47R). Redacting a header
// strips hidden property names exactly as it does for an entity, and records
// them in Redacted, so a gated header read is never MORE revealing than a
// gated entity read.
func (r *PolicyReader) FilterHeaders(
	ctx context.Context, candidates []store.EntityHeader,
) []store.EntityHeader {
	if len(candidates) == 0 {
		return nil
	}
	byType := make(map[string][]string)
	for _, c := range candidates {
		byType[c.Type] = append(byType[c.Type], c.ID)
	}
	allowed := r.permittedFaces(ctx, byType)

	out := make([]store.EntityHeader, 0, len(candidates))
	probes := make([]*entity.Entity, 0, len(candidates))
	for _, c := range candidates {
		if allowed[c.ID].Contains(c.Face) && FaceAllowed(ctx, r.gate, c.Type, c.Face) {
			out = append(out, c)
			probes = append(probes, headerProbe(c))
		}
	}
	ctx = PrimeTraversals(ctx, r.redact, probes)
	for i, c := range out {
		out[i] = RedactHeader(ctx, r.redact, c)
	}
	return out
}

// FilterRelations implements [Reader]: a relation survives only when BOTH
// endpoints are readable (FROM ∧ TO), as [Resolver.EndpointsReadable]
// decides. That reads headers in one query for the whole batch, so a faced
// endpoint is found at its stored faces, and a content-scoped tail is gated
// at the face it attaches to (RR-2IK76Z). A missing endpoint or a gate error
// hides fail-closed.
func (r *PolicyReader) FilterRelations(ctx context.Context, rels []*entity.Relation) []*entity.Relation {
	if len(rels) == 0 {
		return nil
	}
	readable := r.res.EndpointsReadable(ctx, rels)
	out := make([]*entity.Relation, 0, len(rels))
	for i, rel := range rels {
		if readable[i] {
			out = append(out, rel)
		}
	}
	return out
}

// FilterRelationsStrict implements [Reader] through
// [Resolver.EndpointsReadableErr].
func (r *PolicyReader) FilterRelationsStrict(
	ctx context.Context, rels []*entity.Relation,
) ([]*entity.Relation, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	readable, err := r.res.EndpointsReadableErr(ctx, rels)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.Relation, 0, len(rels))
	for i, rel := range rels {
		if readable[i] {
			out = append(out, rel)
		}
	}
	return out, nil
}

// permittedFaces runs one ReadableFacesMany per distinct type and returns,
// per id, the faces whose row passes the verdict. An id absent from the
// result reads no face. A gate error drops that whole type fail-closed — a
// read-ACL failure must never widen visibility — and is logged loud so
// operators see the cause rather than silently thinner results.
func (r *PolicyReader) permittedFaces(ctx context.Context, byType map[string][]string) map[string]acl.FaceVerdict {
	allowed := make(map[string]acl.FaceVerdict)
	for typeName, ids := range byType {
		verdicts, err := r.gate.ReadableFacesMany(ctx, typeName, ids)
		if err != nil {
			slog.Warn("visibility: ReadableFacesMany failed; dropping type fail-closed",
				"type", typeName, "candidates", len(ids), "err", err)
			continue
		}
		for _, id := range ids {
			if v := verdicts.For(id); !v.None() {
				allowed[id] = v
			}
		}
	}
	return allowed
}

// redacted returns e with hidden properties stripped. When nothing is
// hidden the ORIGINAL face is returned (read-out contract: callers
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
	return Redact(ctx, r.redact, e)
}

// Redact returns e with the properties hidden from the ctx principal
// stripped, per red. When nothing is hidden the ORIGINAL face is
// returned (read-only contract); otherwise a shallow struct copy with a
// fresh filtered Properties map (the redacted() contract above — see its
// godoc for the copy semantics and the body-redaction TODO).
//
// Exported for consumers that hold an already-ROW-GATED, already-loaded
// entity where a type-claimed [Resolver] read doesn't fit — e.g. redacting a
// visible neighbor before deriving its display title (the RR-5N4K35
// title-leak class). Redact performs NO row-gate of its own: callers own
// that decision.
//
// PRECONDITION: e must be a raw store entity, never the output of a prior
// Redact. On the nothing-hidden path the input is returned untouched, so a
// stale [entity.Entity.Redacted] from an earlier pass would survive and
// misreport as this redactor's verdict. Clearing it unconditionally would
// cost the allocation-free identity guarantee above, so the contract is the
// caller's to keep. No production caller stacks readers today (RR-Q1VCKR).
func Redact(ctx context.Context, red FieldRedactor, e *entity.Entity) *entity.Entity {
	if e == nil {
		return nil
	}
	hidden := red.HiddenProperties(ctx, e)
	if len(hidden) == 0 {
		return e
	}
	out := *e
	out.Properties = filterProps(e.Properties, hidden)
	// Record WHICH properties were withheld so a consumer can render
	// "[redacted]" instead of a blank that reads as "never set"
	// (TKT-FJ6END). Names only — the values stay stripped above.
	//
	// Freshly allocated and sorted, never appended to e.Redacted: the
	// shallow copy above aliases the original's slice header, so growing
	// it in place could write into the caller's backing array.
	out.Redacted = slices.Sorted(maps.Keys(hidden))
	return &out
}

// RedactHeader is [Redact] for a content-free [store.EntityHeader].
//
// Verdicts are resolved from the entity TYPE and the ctx principal — the
// production redactors read e.Type and never e.Content (affordances'
// FieldVerdicts resolves against the metamodel's declared fields) — so a
// header yields the same hidden set its entity would. The stand-in Entity
// below exists only to satisfy the [FieldRedactor] signature.
//
// Mirrors Redact's copy semantics: nothing hidden returns the input
// unchanged (no allocation on the no-policy path); otherwise Properties is
// a fresh filtered map and Redacted a freshly sorted slice, never appended
// to the input's (which may alias a caller's backing array).
func RedactHeader(ctx context.Context, red FieldRedactor, h store.EntityHeader) store.EntityHeader {
	hidden := red.HiddenProperties(ctx, headerProbe(h))
	if len(hidden) == 0 {
		return h
	}
	out := h
	out.Properties = filterProps(h.Properties, hidden)
	out.Redacted = slices.Sorted(maps.Keys(hidden))
	return out
}

// headerProbe is the entity a redactor sees for a header. It carries the face:
// a `when:` that cannot be answered on a named face must see that it is one.
func headerProbe(h store.EntityHeader) *entity.Entity {
	return &entity.Entity{ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties}
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
