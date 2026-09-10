package datamigration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// MigrationsDir is the project directory that holds data-migration files,
// committed alongside schema.yaml (a migration is operator-authored config).
const MigrationsDir = "migrations"

// File is one parsed data-migration file: an edge from one shape hash to
// another, with the transforming steps. Both projections are EMBEDDED
// (amendment A2) so the file is self-contained: the chain resolver compares
// the store's current projection against FromProjection without any
// historical-schema source, and plan-time step validation has real shapes to
// check targets against. Each hash is integrity-checked against its embedded
// projection at parse time, so a hand-edited projection cannot silently
// diverge from the hash the chain keys on.
type File struct {
	// Name is the file's base name (e.g. "0001-rename-status.yaml").
	// Files run in lexicographic Name order; the marker's applied list
	// records Names.
	Name string

	From           string
	To             string
	FromProjection metamodel.ShapeProjection
	ToProjection   metamodel.ShapeProjection
	Description    string
	Steps          []Step
}

// fileYAML is the on-disk schema. Projections are generic maps here: they
// were serialized from JSON (metamodel.ShapeProjection.JSON) and are
// re-marshaled through JSON on load, so the YAML layer never needs yaml tags
// on metamodel types.
type fileYAML struct {
	From           string         `yaml:"from"`
	To             string         `yaml:"to"`
	FromProjection map[string]any `yaml:"from_projection"`
	ToProjection   map[string]any `yaml:"to_projection"`
	Description    string         `yaml:"description,omitempty"`
	Steps          []yaml.Node    `yaml:"steps"`
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ParseFile parses and validates one migration file. Every error names the
// file; step errors name the step index and kind too.
func ParseFile(name string, data []byte) (*File, error) {
	var raw fileYAML
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("datamigration: %s: %w", name, err)
	}

	if !hashRe.MatchString(raw.From) {
		return nil, fmt.Errorf("datamigration: %s: `from` is not a shape hash (64 hex chars)", name)
	}
	if !hashRe.MatchString(raw.To) {
		return nil, fmt.Errorf("datamigration: %s: `to` is not a shape hash (64 hex chars)", name)
	}
	if raw.From == raw.To {
		return nil, fmt.Errorf("datamigration: %s: `from` and `to` are the same shape hash", name)
	}

	fromProj, err := projectionFromYAML(raw.FromProjection)
	if err != nil {
		return nil, fmt.Errorf("datamigration: %s: from_projection: %w", name, err)
	}
	toProj, err := projectionFromYAML(raw.ToProjection)
	if err != nil {
		return nil, fmt.Errorf("datamigration: %s: to_projection: %w", name, err)
	}
	// Integrity: the hash keys the chain; the projection feeds validation
	// and free-edge comparison. They MUST agree or the file is corrupt.
	if h := fromProj.Hash(); h != raw.From {
		return nil, fmt.Errorf("datamigration: %s: from_projection hashes to %s, not the declared `from` %s", name, h, raw.From)
	}
	if h := toProj.Hash(); h != raw.To {
		return nil, fmt.Errorf("datamigration: %s: to_projection hashes to %s, not the declared `to` %s", name, h, raw.To)
	}

	f := &File{
		Name:           name,
		From:           raw.From,
		To:             raw.To,
		FromProjection: fromProj,
		ToProjection:   toProj,
		Description:    raw.Description,
	}
	for i := range raw.Steps {
		step, err := parseStep(&raw.Steps[i])
		if err != nil {
			return nil, fmt.Errorf("datamigration: %s: step %d: %w", name, i+1, err)
		}
		f.Steps = append(f.Steps, step)
	}
	for i, s := range f.Steps {
		if err := s.Validate(fromProj, toProj); err != nil {
			return nil, fmt.Errorf("datamigration: %s: step %d (%s): %w", name, i+1, s.Kind(), err)
		}
	}
	if err := validateStepOrder(name, f.Steps); err != nil {
		return nil, err
	}
	if err := validateDeltasResolved(name, f); err != nil {
		return nil, err
	}
	return f, nil
}

// resolvingSteps maps each TierMigration delta kind to the step kinds that can
// answer it. A kind mapped to an EMPTY list is a known, reviewed gap.
//
// This is the explicit mapping between what the classifier can DEMAND and what
// the step vocabulary can DELIVER. Keeping it in one table is the point: the
// defect that motivated it (BUG-TMGWIN) existed because detection and
// remediation were maintained as separate lists, each looking complete on its
// own. `TestResolvingSteps_CoversEveryMigrationDeltaKind` fails when
// CompareShapes learns a kind that is not listed here, so a new delta kind
// cannot ship detection without either a resolving step or a deliberate,
// visible exemption.
var resolvingSteps = map[string][]string{
	"bare_face_introduced": {"confirm_face"},

	// Deliberately unresolved, both filed.
	//
	// bare_face_changed (TKT-L3P8I6): repointing between two EXISTING faces is
	// a different problem from adopting the first one. The type already has
	// named-face rows, so the newly-bare face may already be occupied — a
	// collision `rename_face` detects and refuses, and that `confirm_face`
	// does not look for. Enforcing confirm_face here would also break the
	// legitimate rename_face-based migrations that handle it today.
	//
	// bare_face_removed (TKT-1YBNQJ): going faced → flat keeps every bare
	// row's content; what is lost is the meaning attached to the coordinate,
	// not the data.
	"bare_face_changed": {},
	"bare_face_removed": {},

	// Property- and relation-level kinds, listed but NOT enforced. These are
	// answered by a step the operator chooses between (map_values vs. a drop,
	// convert vs. lua) or, for the relation kinds, by hand — the generator
	// says as much when it drafts them. Requiring one specific step would be
	// wrong, and requiring "any step at all" would be a check that a
	// well-formed no-op passes.
	//
	// The face kinds differ because there is exactly one right answer and it
	// is cheap to state, which is what makes enforcing it reasonable.
	"enum_values_replaced":           {},
	"property_type_changed":          {},
	"property_format_changed":        {},
	"property_list_changed":          {},
	"relation_endpoint_narrowed":     {},
	"relation_cardinality_tightened": {},
	"relation_symmetry_changed":      {},
}

// validateDeltasResolved refuses a file whose own edge raises a needs-migration
// delta that the file does not answer.
//
// Without this, a migration satisfies the gate by DECLARING the right `to` hash
// while its steps do nothing — which is exactly how BUG-TMGWIN reached
// production: the classifier demanded a confirmation, and a file with no steps
// at all discharged that demand by hashing correctly.
//
// It is also what makes the generator's drafts honest. Because an unconfirmed
// file cannot parse, `rela migrate gen` must emit a real `confirm_face` step
// rather than a commented suggestion, so the operator reviews a step that will
// actually run instead of one they can ignore into the old behavior.
//
// Only kinds with a non-empty entry in [resolvingSteps] are enforced. The
// deltas are recomputed from the file's OWN embedded projections, so the check
// needs no live schema and cannot drift from what the file claims to do.
func validateDeltasResolved(name string, f *File) error {
	present := map[string]map[string]bool{}
	for _, s := range f.Steps {
		if cf, ok := s.(*confirmFaceStep); ok {
			if present[s.Kind()] == nil {
				present[s.Kind()] = map[string]bool{}
			}
			present[s.Kind()][cf.Entity] = true
		}
	}

	report := metamodel.CompareShapes(f.FromProjection, f.ToProjection)
	for _, d := range report.ByTier(metamodel.TierMigration) {
		wanted := resolvingSteps[d.Kind]
		if len(wanted) == 0 {
			continue
		}
		satisfied := false
		for _, kind := range wanted {
			if present[kind][d.Subject] {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return fmt.Errorf(
				"datamigration: %s: the schema change this file spans needs a %s step for %q, and the file "+
					"has none — %s. Without it the migration would apply, report the schema in sync, and "+
					"leave the change unconfirmed",
				name, strings.Join(wanted, " or "), d.Subject, d.Detail)
		}
	}
	return nil
}

// validateStepOrder checks step ordering WITHIN one file. It is deliberately
// narrow, and the narrowness is worth stating so nobody reads it as a general
// guarantee.
//
// One rule: confirm_face reads a property's values to state which face the
// existing rows become, so a drop_property that erases that property must not
// come first. Ordering is the operator's to choose, but this order is never
// intentional — it leaves the step reporting on nothing, which turns an
// informed confirmation back into the empty ceremony it exists to replace.
//
// What it does NOT catch: the same pair split across two files (each file is
// parsed alone; that case is caught incidentally, because the second file's
// from-shape no longer declares the property), or a lua/map_values step that
// changes the values out from under the confirmation. Catching those means
// reading scripts and tracking value flow between steps — a value-provenance
// feature, not an ordering check. The map_values case is why confirm_face's
// uncovered-row note describes what it saw rather than asserting a cause.
func validateStepOrder(name string, steps []Step) error {
	type dropped struct {
		entity, property string
		at               int
	}
	var drops []dropped
	for i, s := range steps {
		switch step := s.(type) {
		case *dropPropertyStep:
			drops = append(drops, dropped{entity: step.Entity, property: step.Property, at: i + 1})
		case *confirmFaceStep:
			for _, d := range drops {
				if d.entity == step.Entity && d.property == step.Property {
					return fmt.Errorf(
						"datamigration: %s: step %d (confirm_face) reads %s.%s, but step %d (drop_property) "+
							"already removed it — confirm_face must come first, or it would report on no "+
							"values and confirm nothing",
						name, i+1, step.Entity, step.Property, d.at)
				}
			}
		}
	}
	return nil
}

// projectionFromYAML converts the YAML-decoded generic map back into a
// ShapeProjection via its JSON form.
func projectionFromYAML(m map[string]any) (metamodel.ShapeProjection, error) {
	if len(m) == 0 {
		return metamodel.ShapeProjection{}, errors.New("missing (a migration file must embed the projection — regenerate with `rela migrate gen`)")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return metamodel.ShapeProjection{}, err
	}
	return metamodel.ShapeProjectionFromJSON(data)
}

// projectionToYAML converts a projection to the generic map the YAML encoder
// serializes (the inverse of projectionFromYAML).
func projectionToYAML(p metamodel.ShapeProjection) (map[string]any, error) {
	data, err := p.JSON()
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadDir parses every *.yaml/*.yml file in the project's migrations/
// directory, sorted by name (the chain order). A missing directory is an
// empty chain, not an error.
func LoadDir(fsys fs.FS) ([]*File, error) {
	entries, err := fs.ReadDir(fsys, MigrationsDir)
	if err != nil {
		// Only a MISSING directory is an empty chain. Anything else
		// (permissions, not-a-directory) must surface: silently treating an
		// unreadable migrations/ as "nothing to migrate" would let the gate
		// report an unresolvable state while real files sit unread.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("datamigration: read %s/: %w", MigrationsDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := path.Ext(e.Name()); ext != ".yaml" && ext != ".yml" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var files []*File
	for _, n := range names {
		data, err := fs.ReadFile(fsys, path.Join(MigrationsDir, n))
		if err != nil {
			return nil, fmt.Errorf("datamigration: read %s: %w", n, err)
		}
		f, err := ParseFile(n, data)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}
