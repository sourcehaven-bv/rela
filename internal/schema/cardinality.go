package schema

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CardinalityViolation represents a cardinality constraint violation.
//
// Face is set when the bound was counted per face: an outgoing bound of a
// content-scoped relation. Any other bound is counted once per family and
// reported once, with the zero face (BUG-95W7MV).
type CardinalityViolation struct {
	EntityID     string
	Face         entity.Face `json:",omitempty"`
	RelationType string
	Constraint   string // "min_outgoing", "max_outgoing", "min_incoming", "max_incoming"
	Required     int
	Actual       int
}

// IsMin reports whether the violated bound was a minimum. Constraint is a
// closed set of four values, so this is the whole discrimination every
// renderer needs.
func (v CardinalityViolation) IsMin() bool {
	return strings.HasPrefix(v.Constraint, "min_")
}

// Message renders the violation as a sentence, WITHOUT the entity id.
//
// The id is omitted because its placement differs per surface: the CLI
// prefixes it ("REQ-002 must have at least ..."), while the MCP payload
// carries it in its own JSON field beside this text. Callers that want the
// CLI form print the id and this, in that order.
//
// It lives on the type so the three surfaces that render cardinality —
// `rela analyze cardinality`, `rela validate --check cardinality`, and the
// MCP analyze_cardinality tool — cannot drift apart in wording the way the
// checks themselves had (TKT-CICJSN).
func (v CardinalityViolation) Message() string {
	if v.IsMin() {
		return fmt.Sprintf("must have at least %d '%s' relation(s), has %d",
			v.Required, v.RelationType, v.Actual)
	}
	return fmt.Sprintf("has more than %d '%s' relation(s): %d",
		v.Required, v.RelationType, v.Actual)
}

// CardinalityReader is the read capability [CheckCardinality] requires, declared
// here at the call site rather than taking `store.Store`.
//
// This lives in `schema` rather than `analysis` so that every consumer can
// reach it: the CLI (through `analysis`), MCP, and data-entry. `internal/mcp`
// may not import `internal/analysis` (arch-lint), so the single implementation
// had to land somewhere all of them already depend on (TKT-CICJSN).
//
// The reader is the gate (TKT-5LW875). A count is folded only from the edges
// ListRelationsStrict yields, so a gated reader, which keeps an edge only when
// [visibility.Resolver.EndpointsReadableErr] finds both endpoints readable,
// yields counts of visible edges only: a neighbor the principal cannot read is
// not counted, and the reported count cannot reveal it. The method is the
// STRICT form because a gate fault must fail the check: a tolerant gated read
// hides the edges it cannot judge, and a count over a thinned set invents min
// violations. The CLI runs under operator trust and passes [Ungated] over the
// raw store, which gets raw counts on the same code path.
//
// What ListEntities yields likewise defines the subject population, including
// how much of the AllStates request it honors. A reader that composes a
// `store.GraphQuery` collapses each id to one world prime, so the check sees
// fewer subjects and reports fewer violations. That is the safe direction
// (missed, never invented), but per-face coverage is a property of the reader,
// not a guarantee of this function (RR-16R183).
type CardinalityReader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelationsStrict(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// UngatedReader is the read surface of a reader with no gate: a raw store or
// an allow-all reader.
type UngatedReader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Ungated adapts a reader with no gate to [CardinalityReader]. Its
// ListRelations cannot hide an edge, so it is already strict. Only an
// operator-trust caller (the CLI) may pass a raw store here; a
// principal-facing surface passes its gated reader instead.
func Ungated(r UngatedReader) CardinalityReader { return ungated{r} }

type ungated struct{ UngatedReader }

func (u ungated) ListRelationsStrict(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return u.ListRelations(ctx, q)
}

// ListEntityHeaders forwards the wrapped reader's header path when it has
// one, so wrapping a store does not turn a header scan into a body scan.
func (u ungated) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	return store.ListEntityHeaders(ctx, u.UngatedReader, q)
}

// cardinalitySpec is one direction of a relation's cardinality constraints:
// the subject population (which entity types are checked), the count
// direction and granularity, the min/max bounds with their constraint labels,
// and the relation label reported on violations (the inverse id for the
// incoming side, when declared).
type cardinalitySpec struct {
	relName      string
	direction    store.Direction
	subjectTypes []string // relDef.From (outgoing) / relDef.To (incoming)
	// perFace counts edges per (From, FromFace): the outgoing side of a
	// content-scoped relation (TKT-4Y6CMV). A content-scoped edge belongs to
	// one state on its tail side, so its bound is a claim about that state;
	// counting by bare id would let one face's edge satisfy another face's
	// min_outgoing. Every other bound is a claim about the whole family and
	// is counted once per id.
	perFace       bool
	minBound      *int // nil or 0 disables the min check
	maxBound      *int // nil disables the max check; 0 forbids any edge
	minConstraint string
	maxConstraint string
	relationLabel string // violation display label; inverse id on the incoming side
}

func (spec cardinalitySpec) minActive() bool { return spec.minBound != nil && *spec.minBound > 0 }
func (spec cardinalitySpec) maxActive() bool { return spec.maxBound != nil }
func (spec cardinalitySpec) active() bool    { return spec.minActive() || spec.maxActive() }

// key returns the subject rel counts toward under spec.
func (spec cardinalitySpec) key(rel *entity.Relation) edgeKey {
	switch {
	case spec.direction == store.DirectionIncoming:
		return edgeKey{id: rel.To}
	case spec.perFace:
		return edgeKey{id: rel.From, face: rel.FromFace}
	default:
		return edgeKey{id: rel.From}
	}
}

// CheckCardinality checks every cardinality constraint the metamodel
// declares, restricted to scope (nil scope checks everything). It is
// [CheckCardinalityFindings] without the subject headers.
func CheckCardinality(
	ctx context.Context, r CardinalityReader, meta *metamodel.Metamodel, scope map[string]bool,
) ([]CardinalityViolation, error) {
	findings, err := CheckCardinalityFindings(ctx, r, meta, scope)
	if err != nil {
		return nil, err
	}
	// Non-nil even when empty: JSON callers serialize Details as [], not null.
	violations := make([]CardinalityViolation, 0, len(findings))
	for _, f := range findings {
		violations = append(violations, f.CardinalityViolation)
	}
	return violations, nil
}

// CardinalityFinding is a violation plus the header of the row it is about:
// the violating face for a per-face bound, otherwise the family's first face
// in face order. A renderer that shows the subject's type or title takes them
// from Subject rather than reading the row again.
type CardinalityFinding struct {
	CardinalityViolation
	Subject store.EntityHeader
}

// CheckCardinalityFindings is the checker behind [CheckCardinality].
//
// Cost is independent of the number of subjects: one ListRelationsStrict query
// per relation with an active bound (serving both directions, and narrowed to
// the scope ids when a scope is given), one header scan per subject type, and
// an in-memory count per subject (TKT-5LW875). Edges are folded as they
// stream, never held.
//
// Output is ordered by relation name, then outgoing before incoming, min
// before max, then subject id and face, all in natural order.
//
// A read error fails the run loudly with a wrapped error and NO violations.
// Reporting around a failed read would fabricate violations: a backend outage
// reads as count 0, which for a min bound looks exactly like missing
// relations (TKT-RNBLAC).
func CheckCardinalityFindings(
	ctx context.Context, r CardinalityReader, meta *metamodel.Metamodel, scope map[string]bool,
) ([]CardinalityFinding, error) {
	var findings []CardinalityFinding
	subjects := subjectCache{}
	relNames := slices.Collect(maps.Keys(meta.Relations))
	natsort.Strings(relNames)

	for _, relName := range relNames {
		relDef := meta.Relations[relName]
		incomingLabel := relName
		if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
			incomingLabel = relDef.Inverse.GetID()
		}
		specs := [2]cardinalitySpec{
			{
				relName: relName, direction: store.DirectionOutgoing, subjectTypes: relDef.From,
				perFace:  relDef.Scope.IsContent(),
				minBound: relDef.MinOutgoing, maxBound: relDef.MaxOutgoing,
				minConstraint: "min_outgoing", maxConstraint: "max_outgoing",
				relationLabel: relName,
			},
			{
				relName: relName, direction: store.DirectionIncoming, subjectTypes: relDef.To,
				minBound: relDef.MinIncoming, maxBound: relDef.MaxIncoming,
				minConstraint: "min_incoming", maxConstraint: "max_incoming",
				relationLabel: incomingLabel,
			},
		}
		if !specs[0].active() && !specs[1].active() {
			continue
		}
		counts, err := countEdges(ctx, r, relName, specs, scope)
		if err != nil {
			return nil, err
		}
		for i, spec := range specs {
			if !spec.active() {
				continue
			}
			v, err := checkCardinalityFor(ctx, r, subjects, spec, counts[i], scope)
			if err != nil {
				return nil, err
			}
			findings = append(findings, v...)
		}
	}
	return findings, nil
}

// edgeKey is the subject an edge counts toward: an id, plus the tail face for
// a per-face count.
type edgeKey struct {
	id   string
	face entity.Face
}

// countEdges streams the edges of one relation type once and folds them into
// per-subject counts for each of specs, aligned with specs. With a scope, the
// query asks only for edges touching a scoped id, in either direction, which
// is every edge a scoped subject's count needs.
func countEdges(
	ctx context.Context, r CardinalityReader, relName string,
	specs [2]cardinalitySpec, scope map[string]bool,
) ([2]map[edgeKey]int, error) {
	counts := [2]map[edgeKey]int{{}, {}}
	q := store.RelationQuery{Type: relName}
	if scope != nil {
		if len(scope) == 0 {
			return counts, nil
		}
		q.EntityIDs = slices.Sorted(maps.Keys(scope))
	}
	for rel, err := range r.ListRelationsStrict(ctx, q) {
		if err != nil {
			return counts, fmt.Errorf("schema: list %q relations: %w", relName, err)
		}
		if rel == nil {
			continue // fail-closed like the visibility filters: never panic on a nil row
		}
		for i, spec := range specs {
			counts[i][spec.key(rel)]++
		}
	}
	return counts, nil
}

// checkCardinalityFor evaluates one direction of one relation against the
// precomputed counts. Min violations are emitted before max violations, so
// the output order matches the historical per-constraint grouping.
func checkCardinalityFor(
	ctx context.Context, r CardinalityReader, cache subjectCache,
	spec cardinalitySpec, counts map[edgeKey]int, scope map[string]bool,
) ([]CardinalityFinding, error) {
	var subjects []cardinalitySubject
	seen := make(map[string]bool)
	for _, subjectType := range spec.subjectTypes {
		headers, err := cache.list(ctx, r, subjectType)
		if err != nil {
			return nil, err
		}
		for _, h := range headers {
			if scope != nil && !scope[h.ID] {
				continue
			}
			key := edgeKey{id: h.ID}
			if spec.perFace {
				key.face = h.Face
			} else if seen[h.ID] {
				continue // a family-wide count: one subject per id
			}
			seen[h.ID] = true
			subjects = append(subjects, cardinalitySubject{header: h, face: key.face, count: counts[key]})
		}
	}

	var findings []CardinalityFinding
	if spec.minActive() {
		findings = spec.outOfBound(findings, subjects, spec.minConstraint, *spec.minBound,
			func(count int) bool { return count < *spec.minBound })
	}
	if spec.maxActive() {
		findings = spec.outOfBound(findings, subjects, spec.maxConstraint, *spec.maxBound,
			func(count int) bool { return count > *spec.maxBound })
	}
	return findings, nil
}

// cardinalitySubject is one counted subject: its header, and its face when
// the bound is counted per face.
type cardinalitySubject struct {
	header store.EntityHeader
	face   entity.Face
	count  int
}

// outOfBound appends a finding of constraint to dst for each subject whose
// count breaks the bound.
func (spec cardinalitySpec) outOfBound(
	dst []CardinalityFinding, subjects []cardinalitySubject,
	constraint string, bound int, breaks func(count int) bool,
) []CardinalityFinding {
	for _, sub := range subjects {
		if breaks(sub.count) {
			dst = append(dst, CardinalityFinding{
				CardinalityViolation: CardinalityViolation{
					EntityID:     sub.header.ID,
					Face:         sub.face,
					RelationType: spec.relationLabel,
					Constraint:   constraint,
					Required:     bound,
					Actual:       sub.count,
				},
				Subject: sub.header,
			})
		}
	}
	return dst
}

// subjectCache holds the subject headers of each entity type, so a type that
// is the subject of several bounds is scanned once per check.
type subjectCache map[string][]store.EntityHeader

// list returns the headers of every stored face of typeName, ordered by id
// (naturally) and then face, so the output does not depend on the backend's
// scan order.
func (c subjectCache) list(ctx context.Context, r CardinalityReader, typeName string) ([]store.EntityHeader, error) {
	if hs, ok := c[typeName]; ok {
		return hs, nil
	}
	hs := make([]store.EntityHeader, 0)
	for h, err := range store.ListEntityHeaders(ctx, r, store.EntityQuery{Type: typeName, AllStates: true}) {
		if err != nil {
			return nil, fmt.Errorf("schema: list %q cardinality subjects: %w", typeName, err)
		}
		hs = append(hs, h)
	}
	slices.SortStableFunc(hs, func(a, b store.EntityHeader) int {
		if a.ID != b.ID {
			if natsort.Less(a.ID, b.ID) {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Face, b.Face)
	})
	c[typeName] = hs
	return hs, nil
}
