package analysis

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StateFinding is one content-state integrity finding (TKT-DOFYR1).
type StateFinding struct {
	// Code is the finding class:
	//   - "undeclared-face": rows exist in a state no metamodel
	//     declaration accounts for. The declared set is consulted PER
	//     ENTITY TYPE (`entities.<type>.faces`, TKT-WAV8XP), never
	//     flattened: a row stored under `draft` on a type that declares
	//     no faces is exactly the stranded data this reports, even
	//     when some OTHER type declares `draft`. Remedy is the future
	//     data migration system (FEAT-T3EF5A, DEC-0VGTF3) — detection
	//     only.
	//   - "bare-row-on-faced-type": rows stored at the bare id on a type
	//     that declares `faces:`. No world's chain names the bare id
	//     (BUG-HC6I2T), so the rows are unreachable. Remedy is
	//     `rela migrate adopt-face` for that type — detection only.
	//   - "unknown-entity-type": rows whose entity type the metamodel does
	//     not define, so no coordinate on them is declared. Reported apart
	//     from the two above because the fault is the TYPE, and neither of
	//     their remedies can apply: adopt-face resolves its mapping
	//     against the type's declared properties, which do not exist.
	//   - "state-type-mismatch": rows of one entity disagree about its
	//     type. The write path refuses this at every face, so it can
	//     only come from disk edits, which the load path tolerates.
	//
	// There is no "headless-family" finding. It reported a family with no
	// zero-coordinate row, which was corrupt while one face was privileged
	// by storage and is the ORDINARY shape of a faced entity now
	// (BUG-HC6I2T) — the write path mandates it. "bare-row-on-faced-type"
	// does not restore it under another name: that check required a bare row
	// to exist, this one reports one that nothing can reach. Different
	// predicates over overlapping data, not inverses.
	Code string `json:"code"`
	// Subject is what an operator acts on, which differs per code: the face
	// value (undeclared-face), the entity type (bare-row-on-faced-type,
	// unknown-entity-type) or the bare entity id (family findings). It is
	// never empty — a finding whose subject could not be named would give a
	// reader nothing to search for.
	Subject string `json:"subject"`
	// Count is the number of affected rows.
	Count int `json:"count"`
	// Examples lists up to [maxStateExamples] affected state references
	// in their boundary serialization. A BARE row serializes as the plain
	// entity id, with no `@face` suffix, so examples for
	// "bare-row-on-faced-type" are indistinguishable from entity ids —
	// which is correct, since that is the address the row is stored at.
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
	face entity.Face
	typ  string
}

// stateFamily groups one bare id's rows during the CheckStates scan.
//
// famType is read from the FIRST row seen, whichever face that is: every row
// of a family shares its type, so any of them answers, and no face is
// privileged (BUG-HC6I2T). Which row is first does not matter — a divergence
// is reported the same way whichever side of it is taken as the baseline.
type stateFamily struct {
	famType string
	states  []stateRow
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
			// Classified as an incomplete scan so every caller treats an
			// unreadable file the same way, whichever analysis hit it
			// first (BUG-4KPN2M).
			return nil, nil, &IncompleteScanError{Op: "list entity states", Err: iterErr}
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
		if f.famType == "" {
			f.famType = h.Type
		}
		f.states = append(f.states, stateRow{face: h.Face, typ: h.Type})
	}
	sort.Strings(order)
	return families, order, nil
}

// faceStatus is why a row's coordinate is, or is not, declared. A bare
// bool cannot carry this: three of the four answers are faults with
// DIFFERENT remedies, and a caller holding only "not declared" has to guess
// which sentence to print. Guessing is how a row of an undefined type came
// to be told it lived on a type declaring `faces:` and to try adopt-face,
// which cannot resolve a type the schema never defined.
type faceStatus int

const (
	// faceOK: the coordinate is declared for this type.
	faceOK faceStatus = iota
	// faceUnknownType: the metamodel does not define the row's type at all,
	// so no coordinate on it is declared and the TYPE is the fault.
	faceUnknownType
	// faceUndeclared: the type exists and does not declare this named face.
	faceUndeclared
	// faceBareOnFaced: the type declares `faces:`, so the bare id names none
	// of them.
	faceBareOnFaced
)

// faultKey identifies one aggregated coordinate fault: the fault itself plus
// the thing an operator would act on to clear it.
//
// The subject is not the same field for every status, because the remedies are
// not the same shape. `undeclared-face` is remedied per FACE (rename it, or
// drop the rows), while `bare-row-on-faced-type` and `unknown-entity-type` are
// remedied per TYPE — adopt-face takes `--entity <type>` and a mapping over
// that type's own property, so one finding spanning two types would describe
// two different repairs.
//
// Keying everything by face merged every faced type in a project into a single
// finding with an empty subject, whose [maxStateExamples] cap could then hide a
// whole type from the operator.
type faultKey struct {
	status  faceStatus
	subject string
}

// faceStatusOf classifies the face p stored on entityType (TKT-WAV8XP).
//
// It reads the METAMODEL directly, not a compiled world scope. The
// undeclared-face check is about DECLARATIONS, not about worlds, so it
// must keep working on a project whose `worlds:` block is malformed —
// coupling it to world compilation would make the stranded-data report
// disappear exactly when the schema is broken.
//
// The zero face is declared only by a type that declares NO faces, whose
// single state lives there and has no name. On a type declaring `faces:` a
// bare row is stranded: since BUG-HC6I2T removed the headless-state
// invariant, a declared face no longer requires a zero-coordinate sibling
// and no world's chain names the bare id. This used to report every zero
// face as declared, on the pre-BUG-HC6I2T reasoning that "every entity has
// one by construction (the bare id addresses it)".
func (s *Service) faceStatusOf(entityType string, p entity.Face) faceStatus {
	// GetEntityDef, not a raw map index: the write path does not
	// canonicalize e.Type, so a stored row legitimately carries an alias.
	// Indexing Entities directly would report every state of an
	// alias-typed entity as undeclared — a false stranded-data finding.
	def, ok := s.deps.Meta.GetEntityDef(entityType)
	if !ok {
		return faceUnknownType
	}
	if p.IsDefault() {
		if len(def.Faces) == 0 {
			return faceOK
		}
		return faceBareOnFaced
	}
	if _, declared := def.Faces[p.String()]; declared {
		return faceOK
	}
	return faceUndeclared
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

	// Aggregate the coordinate faults, keyed by [faultKey].
	type faultAgg struct {
		count    int
		examples []string
	}
	byFault := make(map[faultKey]*faultAgg)
	var faults []faultKey

	var findings []StateFinding
	for _, id := range order {
		f := families[id]
		if len(f.states) == 0 {
			continue
		}
		var mismatched []string
		for _, st := range f.states {
			// Subtract the DECLARED set, per entity type. A face
			// declared by type A but stored on type B is undeclared for
			// B and still reports — flattening the sets would hide the
			// worst case, since a type declaring no faces contributes
			// its default state to every world, making such a row
			// reachable through no world at all.
			if status := s.faceStatusOf(st.typ, st.face); status != faceOK {
				k := faultKey{status: status, subject: st.typ}
				if status == faceUndeclared {
					k.subject = st.face.String()
				}
				agg := byFault[k]
				if agg == nil {
					agg = &faultAgg{}
					byFault[k] = agg
					faults = append(faults, k)
				}
				agg.count++
				if len(agg.examples) < maxStateExamples {
					agg.examples = append(agg.examples, entity.FormatStateRef(id, st.face))
				}
			}
			if st.typ != f.famType {
				mismatched = append(mismatched, entity.FormatStateRef(id, st.face))
			}
		}
		// One finding per FAMILY for each class, aggregated like
		// undeclared-face, so a JSON consumer grouping by Subject
		// sees one row per entity.
		if len(mismatched) > 0 {
			findings = append(findings, StateFinding{
				Code: "state-type-mismatch", Subject: id, Count: len(mismatched),
				Examples: mismatched[:min(len(mismatched), maxStateExamples)],
				Detail: fmt.Sprintf("%d state(s) of entity %s diverge from its type %q",
					len(mismatched), id, f.famType),
			})
		}
	}

	// Sorted by (status, subject) so a project with several faults reports
	// them in a stable order — a report that reshuffles between runs is one
	// an operator cannot diff.
	slices.SortFunc(faults, func(a, b faultKey) int {
		if a.status != b.status {
			return int(a.status) - int(b.status)
		}
		return strings.Compare(a.subject, b.subject)
	})
	coordinate := make([]StateFinding, 0, len(faults))
	for _, k := range faults {
		agg := byFault[k]
		coordinate = append(coordinate, coordinateFinding(k, agg.count, agg.examples))
	}

	// Coordinate findings first (the headline class), then the
	// per-family findings in id order.
	return append(coordinate, findings...), nil
}

// coordinateFinding renders one aggregated coordinate fault.
//
// Each status gets its own code, subject and remedy. They were one code keyed
// on the face until a review found a row of an UNDEFINED type being told it sat
// on a type declaring `faces:` and to fix it with adopt-face — which resolves
// its mapping against the type's declared properties, so the operator's
// command could not have run.
//
// Split out of [Service.CheckStates] to keep that function under the cognitive
// complexity limit; it is a pure switch over the key and holds no state.
func coordinateFinding(k faultKey, count int, examples []string) StateFinding {
	f := StateFinding{Subject: k.subject, Count: count, Examples: examples}
	switch k.status {
	case faceUnknownType:
		f.Code = "unknown-entity-type"
		f.Detail = fmt.Sprintf("stored with entity type %q, which the metamodel does not define, "+
			"so nothing declares its properties, faces or relations and a world scopes it by "+
			"no rule but rule 1; declare the type or remove the rows (detection only)", k.subject)
	case faceBareOnFaced:
		f.Code = "bare-row-on-faced-type"
		f.Detail = fmt.Sprintf("stored at the bare id on type %q, which declares `faces:`, so no "+
			"world's chain names it; adopt the rows into a face with "+
			"`rela migrate adopt-face --entity %s` (detection only)", k.subject, k.subject)
	case faceUndeclared:
		f.Code = "undeclared-face"
		f.Detail = fmt.Sprintf("stored under face %q, which no metamodel declaration accounts for; "+
			"the data-migration system is the remedy (detection only)", k.subject)
	case faceOK:
		// Unreachable: faceOK never enters the aggregation.
	}
	return f
}
