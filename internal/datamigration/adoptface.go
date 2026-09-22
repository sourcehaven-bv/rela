package datamigration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// adoptTool attributes the repair to itself rather than to the migration
// runner: the two are separate operator actions and an audit reader must be
// able to tell a schema-change migration from a stranded-row repair.
const adoptTool = "data-adopt-face"

// adoptTriggeredBy labels version rows this command captures. The runner uses
// the migration file's name; there is no file here, so the command names
// itself.
const adoptTriggeredBy = "migrate adopt-face"

// faceAdoption is the row-level move shared by the migrate_face STEP and the
// adopt-face COMMAND (TKT-FTOENU).
//
// The two differ only in what authorizes them. The step runs across a
// faces_introduced edge and proves its mapping against the migration file's
// embedded projections; the command runs with no shape change at all, against
// the live metamodel, and repairs rows that are already stranded. The move
// itself — which rows qualify, what an occupied destination means, what makes
// a re-run converge — is one behavior, so it is written once here.
type faceAdoption struct {
	entityType string
	property   string
	mapping    map[string]string
}

// adoptionPlan is the read-only half of an adoption: which rows move where,
// and which were left behind with the reason why.
//
// Planning is separated from writing so a dry-run is the same code path as an
// apply minus the writes — the count an operator reviews is the count that
// will move, not an estimate computed by a second implementation.
type adoptionPlan struct {
	moves    []faceMove
	unmapped map[string]int
}

// plan collects the zero-coordinate rows this adoption would move.
//
// Rows already on a named face are skipped rather than reported: on the step's
// path they are a previous run's work, and on the command's path they are the
// healthy majority. Either way they are not stranded, which is the only thing
// this operates on.
func (a faceAdoption) plan(ctx context.Context, st store.Store) (*adoptionPlan, error) {
	p := &adoptionPlan{unmapped: map[string]int{}}
	q := store.EntityQuery{Type: a.entityType, AllStates: true}
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		if !e.Face.IsDefault() {
			continue
		}
		v, ok := e.Properties[a.property].(string)
		if !ok {
			p.unmapped[""]++
			continue
		}
		target, ok := a.mapping[v]
		if !ok {
			// Reported, not guessed at. The note describes what was seen and
			// not why: a stale stored value and an earlier step in the same
			// file produce the identical observation.
			p.unmapped[v]++
			continue
		}
		p.moves = append(p.moves, faceMove{e: e, to: target})
	}
	return p, nil
}

// notes renders the left-behind rows as operator-facing lines.
func (p *adoptionPlan) notes(property string) []string {
	out := make([]string, 0, len(p.unmapped))
	for _, value := range slices.Sorted(maps.Keys(p.unmapped)) {
		label := value
		if label == "" {
			label = "(unset or non-string)"
		}
		out = append(out, fmt.Sprintf(
			"%d row(s) with %s = %s are not covered by the mapping and stay at the zero coordinate",
			p.unmapped[value], property, label))
	}
	return out
}

// apply performs the moves.
//
// Delegates to [applyMoves], which is also what the relocating migration steps
// run: a face move is one behavior whichever operator action reached it, and
// the properties that make it safe — the batched transaction around each
// create-then-delete pair, and carrying the source row's outgoing edges across
// so the delete does not destroy them (BUG-TOX8U4) — are not ones this path
// may quietly do without.
func (p *adoptionPlan) apply(ctx context.Context, st store.Store) error {
	return applyMoves(ctx, st, p.moves)
}

// AdoptDeps are the collaborators [Adopt] needs.
type AdoptDeps struct {
	Store    store.Store
	Meta     *metamodel.Metamodel
	Audit    audit.Audit
	Versions VersionCapture
	Lock     MigrationLock
}

// AdoptRequest names the rows to adopt and where they go.
type AdoptRequest struct {
	Entity   string
	Property string
	Mapping  map[string]string
	Apply    bool
}

// AdoptResult reports what moved (or would move).
type AdoptResult struct {
	Affected int
	Notes    []string
	Applied  bool
}

// Adopt moves stranded zero-coordinate rows onto a declared face for a type
// that ALREADY declares faces (TKT-FTOENU).
//
// # Why this is not a migration file
//
// A migration file is an edge between two shape hashes. This repair has no
// edge: the schema is already correct and the data is behind it, so there is
// no delta to resolve and no marker to advance. The rows got stranded by
// history — data written before the type declared faces:, a hand-edited file,
// an import, a seed — and the write path has refused to produce more since
// (entitymanager.ErrFaceRequired).
//
// # Why the mapping need not be exhaustive
//
// migrate_face requires a total mapping because it runs in the same file as
// the drop_property that erases the keying values: a value left out becomes
// unrecoverable. Here nothing is dropped. A value left out leaves its rows
// where they already are — still stranded, still reported by `rela analyze`,
// still fixable by re-running with a wider mapping. Refusing the whole
// operation would only stop an operator from fixing the rows they CAN place,
// so unmapped values are reported instead.
//
// Nil: Store, Meta and Lock are rejected; Audit is required because this is a
// raw-store write and an unaudited one is not a sanctioned exception.
func Adopt(ctx context.Context, deps AdoptDeps, req AdoptRequest) (*AdoptResult, error) {
	if deps.Store == nil || deps.Meta == nil || deps.Lock == nil || deps.Audit == nil {
		return nil, errors.New("datamigration: Adopt needs a store, metamodel, audit sink and lock")
	}
	if err := validateAdopt(deps.Meta, req); err != nil {
		return nil, err
	}
	if req.Apply {
		// Same exclusion as a migration apply: bulk row moves must not
		// interleave with a runner, a GC sweep or a gate adoption. Dry-runs
		// are read-only and stay lock-free.
		release, err := deps.Lock.TryAcquire(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}

	p := principal.From(ctx)
	ctx = store.WithAttribution(ctx, store.Attribution{User: p.User, Tool: adoptTool})

	a := faceAdoption{entityType: req.Entity, property: req.Property, mapping: req.Mapping}
	plan, err := a.plan(ctx, deps.Store)
	if err != nil {
		return nil, err
	}
	res := &AdoptResult{Affected: len(plan.moves), Notes: plan.notes(req.Property), Applied: req.Apply}
	if !req.Apply {
		return res, nil
	}

	// The zero-coordinate row is deleted, and on the database backends the
	// sweep cannot reconstruct a row that no longer exists — so capture it
	// synchronously first, exactly as the runner's delete steps do.
	if vc := newCapturer(deps.Versions, deps.Meta, adoptTool, adoptTriggeredBy); vc != nil {
		for _, m := range plan.moves {
			if err := vc.entityDelete(ctx, m.e); err != nil {
				return res, err
			}
		}
	}
	if err := plan.apply(ctx, deps.Store); err != nil {
		return res, err
	}
	auditAdopt(deps.Audit, p, req, res.Affected)
	return res, nil
}

// validateAdopt checks the request against the LIVE metamodel.
//
// The live schema is the right authority precisely because no shape change is
// involved: the faces the rows are moving to are the ones declared right now,
// and a type that declares none has no stranded rows by definition — its zero
// coordinate is where its single state belongs.
func validateAdopt(meta *metamodel.Metamodel, req AdoptRequest) error {
	if req.Entity == "" || req.Property == "" || len(req.Mapping) == 0 {
		return errors.New("datamigration: adopt needs an entity type, a keying property and a non-empty mapping")
	}
	def, ok := meta.GetEntityDef(req.Entity)
	if !ok {
		return fmt.Errorf("datamigration: entity type %q is not declared", req.Entity)
	}
	if len(def.Faces) == 0 {
		return fmt.Errorf("datamigration: entity type %q declares no faces, so its rows are not stranded — "+
			"the zero coordinate is where its single state belongs", req.Entity)
	}
	if _, ok := def.Properties[req.Property]; !ok {
		return fmt.Errorf("datamigration: %s has no property %q to key the move on", req.Entity, req.Property)
	}
	for _, key := range slices.Sorted(maps.Keys(req.Mapping)) {
		face := req.Mapping[key]
		if _, declared := def.Faces[face]; !declared {
			return fmt.Errorf("datamigration: %q is not a face declared on %s (declared: %s)",
				face, req.Entity, strings.Join(slices.Sorted(maps.Keys(def.Faces)), ", "))
		}
	}
	return nil
}

// auditAdopt records the repair: names and counts, never content.
func auditAdopt(sink audit.Audit, p principal.Principal, req AdoptRequest, affected int) {
	pairs := make([]string, 0, len(req.Mapping))
	for _, k := range slices.Sorted(maps.Keys(req.Mapping)) {
		pairs = append(pairs, k+"="+req.Mapping[k])
	}
	sink.Record(audit.Record{
		Time:      time.Now().UTC(),
		Op:        audit.OpDataAdoptFace,
		Principal: p,
		Summary: fmt.Sprintf("adopted %d stranded row(s) of %s onto faces by %s (%s)",
			affected, req.Entity, req.Property, strings.Join(pairs, ", ")),
	})
}
