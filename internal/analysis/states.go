package analysis

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StateFinding is one content-state integrity finding (TKT-DOFYR1).
type StateFinding struct {
	// Code is the finding class:
	//   - "undeclared-pointer": rows exist in a state no metamodel
	//     declaration accounts for. Step 1 has no pointer declarations
	//     at all, so EVERY stored pointer reports here; when the
	//     metamodel gains `pointers:` (Step 2, TKT-WAV8XP) this check
	//     subtracts the declared set. Remedy is the future data
	//     migration system (FEAT-T3EF5A, DEC-0VGTF3) — detection only.
	//   - "headless-family": states exist with no default row (the
	//     write path rejects this; the load path tolerates it on disk).
	//   - "state-type-mismatch": a state's type diverges from its
	//     family's default (same tolerance rationale).
	Code string `json:"code"`
	// Subject is the pointer value (undeclared-pointer) or the bare
	// entity id (family findings).
	Subject string `json:"subject"`
	// Count is the number of affected rows.
	Count int `json:"count"`
	// Examples lists up to [maxStateExamples] affected state references
	// in their boundary serialization.
	Examples []string `json:"examples,omitempty"`
	Detail   string   `json:"detail"`
}

// maxStateExamples bounds the example list per finding: enough to find
// the rows, small enough for a summary line.
const maxStateExamples = 5

// stateRow is the projection of a state header the check needs —
// deliberately not the full EntityHeader, whose live Properties map
// (and, on the fs fallback path, the loaded body behind it) would
// otherwise stay resident for the whole scan.
type stateRow struct {
	pointer entity.Pointer
	typ     string
}

// stateFamily groups one bare id's rows during the CheckStates scan.
type stateFamily struct {
	defaultType string
	hasDefault  bool
	states      []stateRow
}

// collectStateFamilies scans raw storage truth into per-id families,
// scope-filtered on the bare id, returning ids in sorted order. The
// findings are computed AFTER this scan completes, on purpose: no
// backend documents an iteration order for AllStates, so any
// default-before-state assumption during the stream would silently
// drop findings on a backend that yields families interleaved.
func (s *Service) collectStateFamilies(
	ctx context.Context, opts Options,
) (families map[string]*stateFamily, order []string, err error) {
	families = make(map[string]*stateFamily)
	for h, iterErr := range store.ListEntityHeaders(ctx, s.deps.Store, store.EntityQuery{AllStates: true}) {
		if iterErr != nil {
			return nil, nil, fmt.Errorf("analysis: list entity states: %w", iterErr)
		}
		if !inScope(h.ID, opts.Scope) {
			continue
		}
		f := families[h.ID]
		if f == nil {
			f = &stateFamily{}
			families[h.ID] = f
			order = append(order, h.ID)
		}
		if h.Pointer.IsDefault() {
			f.hasDefault = true
			f.defaultType = h.Type
		} else {
			f.states = append(f.states, stateRow{pointer: h.Pointer, typ: h.Type})
		}
	}
	sort.Strings(order)
	return families, order, nil
}

// CheckStates scans RAW storage truth (EntityQuery.AllStates) for
// content-state integrity findings, filtered by scope on the bare id.
//
// Error policy matches CheckCardinality, not the under-count logging of
// the older analyses: this is an integrity check, so a store error
// fails the run loudly rather than quietly reporting "no findings".
func (s *Service) CheckStates(ctx context.Context, opts Options) ([]StateFinding, error) {
	families, order, err := s.collectStateFamilies(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Aggregate per pointer value for the undeclared-pointer findings.
	type ptrAgg struct {
		count    int
		examples []string
	}
	byPointer := make(map[entity.Pointer]*ptrAgg)
	var pointers []entity.Pointer

	var findings []StateFinding
	for _, id := range order {
		f := families[id]
		if len(f.states) == 0 {
			continue
		}
		var mismatched []string
		for _, st := range f.states {
			agg := byPointer[st.pointer]
			if agg == nil {
				agg = &ptrAgg{}
				byPointer[st.pointer] = agg
				pointers = append(pointers, st.pointer)
			}
			agg.count++
			if len(agg.examples) < maxStateExamples {
				agg.examples = append(agg.examples, entity.FormatStateRef(id, st.pointer))
			}
			if f.hasDefault && st.typ != f.defaultType {
				mismatched = append(mismatched, entity.FormatStateRef(id, st.pointer))
			}
		}
		// One finding per FAMILY for each class, aggregated like
		// undeclared-pointer, so a JSON consumer grouping by Subject
		// sees one row per entity.
		if len(mismatched) > 0 {
			findings = append(findings, StateFinding{
				Code: "state-type-mismatch", Subject: id, Count: len(mismatched),
				Examples: mismatched[:min(len(mismatched), maxStateExamples)],
				Detail: fmt.Sprintf("%d state(s) of entity %s diverge from its type %q",
					len(mismatched), id, f.defaultType),
			})
		}
		if !f.hasDefault {
			examples := make([]string, 0, min(len(f.states), maxStateExamples))
			for _, st := range f.states[:min(len(f.states), maxStateExamples)] {
				examples = append(examples, entity.FormatStateRef(id, st.pointer))
			}
			findings = append(findings, StateFinding{
				Code: "headless-family", Subject: id, Count: len(f.states),
				Examples: examples,
				Detail: fmt.Sprintf("entity %s has %d state(s) but no default state — "+
					"the write path rejects this shape; it can only come from disk edits", id, len(f.states)),
			})
		}
	}

	slices.Sort(pointers)
	undeclared := make([]StateFinding, 0, len(pointers))
	for _, p := range pointers {
		agg := byPointer[p]
		undeclared = append(undeclared, StateFinding{
			Code: "undeclared-pointer", Subject: string(p), Count: agg.count,
			Examples: agg.examples,
			Detail: fmt.Sprintf("stored under pointer %q, which no metamodel declaration accounts for; "+
				"the data-migration system is the remedy (detection only)", p),
		})
	}

	// Undeclared-pointer findings first (the headline class), then the
	// per-family findings in id order.
	return append(undeclared, findings...), nil
}
