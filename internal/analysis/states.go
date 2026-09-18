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
	//   - "undeclared-face": rows exist in a state no metamodel
	//     declaration accounts for. The declared set is consulted PER
	//     ENTITY TYPE (`entities.<type>.faces`, TKT-WAV8XP), never
	//     flattened: a row stored under `draft` on a type that declares
	//     no faces is exactly the stranded data this reports, even
	//     when some OTHER type declares `draft`. Remedy is the future
	//     data migration system (FEAT-T3EF5A, DEC-0VGTF3) — detection
	//     only.
	//   - "bare-row-on-faced-type": rows stored at the bare id on a type
	//     that declares `faces:`. The bare id names no declared face
	//     (BUG-HC6I2T), so no world can reach the row. Remedy is a
	//     `migrate_face` step adopting it into a face — detection only.
	//   - "state-type-mismatch": rows of one entity disagree about its
	//     type. The write path refuses this at every face, so it can
	//     only come from disk edits, which the load path tolerates.
	//
	// There is no "headless-family" finding. It reported a family with no
	// zero-coordinate row, which was corrupt while one face was privileged
	// by storage and is the ORDINARY shape of a faced entity now
	// (BUG-HC6I2T) — the write path mandates it. "bare-row-on-faced-type" is
	// its MIRROR, not its return: the removed check wanted a bare row and
	// this one reports the presence of the same thing the same change made
	// meaningless.
	Code string `json:"code"`
	// Subject is the face value (undeclared-face), the bare
	// entity id (family findings), or empty (bare-row-on-faced-type,
	// whose subject IS the zero face and has no name).
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
		if f.famType == "" {
			f.famType = h.Type
		}
		f.states = append(f.states, stateRow{face: h.Face, typ: h.Type})
	}
	sort.Strings(order)
	return families, order, nil
}

// faceDeclared reports whether entityType declares the face p in
// the metamodel (TKT-WAV8XP).
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
// and the bare id names no declared face, so no world can reach the row.
// This used to return true unconditionally, on the pre-BUG-HC6I2T reasoning
// that "every entity has one by construction (the bare id addresses it)".
func (s *Service) faceDeclared(entityType string, p entity.Face) bool {
	// GetEntityDef, not a raw map index: the write path does not
	// canonicalize e.Type, so a stored row legitimately carries an alias.
	// Indexing Entities directly would report every state of an
	// alias-typed entity as undeclared — a false stranded-data finding.
	def, ok := s.deps.Meta.GetEntityDef(entityType)
	if !ok {
		// A state of a type the metamodel does not define declares
		// nothing. Reporting it is right: the row is unreachable.
		return false
	}
	if p.IsDefault() {
		return len(def.Faces) == 0
	}
	_, declared := def.Faces[p.String()]
	return declared
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

	// Aggregate per face value for the undeclared-face findings.
	type ptrAgg struct {
		count    int
		examples []string
	}
	byFace := make(map[entity.Face]*ptrAgg)
	var faces []entity.Face

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
			if !s.faceDeclared(st.typ, st.face) {
				agg := byFace[st.face]
				if agg == nil {
					agg = &ptrAgg{}
					byFace[st.face] = agg
					faces = append(faces, st.face)
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

	slices.Sort(faces)
	undeclared := make([]StateFinding, 0, len(faces))
	for _, p := range faces {
		agg := byFace[p]
		// The zero face gets its own code and its own sentence. Reporting it
		// as `undeclared-face` with an empty Subject would print `face ""`,
		// which names nothing an operator can search for, and the remedy
		// differs: a named undeclared face needs a rename or a delete, a bare
		// row on a faced type needs adopting INTO a face (`migrate_face`).
		if p.IsDefault() {
			undeclared = append(undeclared, StateFinding{
				Code: "bare-row-on-faced-type", Subject: "", Count: agg.count,
				Examples: agg.examples,
				Detail: "stored at the bare id on a type that declares `faces:`, so it names " +
					"no declared face and no world can reach it; adopt it into a face with a " +
					"`migrate_face` migration step (detection only)",
			})
			continue
		}
		undeclared = append(undeclared, StateFinding{
			Code: "undeclared-face", Subject: string(p), Count: agg.count,
			Examples: agg.examples,
			Detail: fmt.Sprintf("stored under face %q, which no metamodel declaration accounts for; "+
				"the data-migration system is the remedy (detection only)", p),
		})
	}

	// Undeclared-face findings first (the headline class), then the
	// per-family findings in id order.
	return append(undeclared, findings...), nil
}
