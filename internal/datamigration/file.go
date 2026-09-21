package datamigration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// MigrationsDir is the project directory that holds data-migration files,
// committed alongside schema.yaml (a migration is operator-authored config).
const MigrationsDir = "migrations"

// File is one parsed data-migration file: a named, ordered list of steps that
// transform stored content, with the before and after schema shapes embedded.
//
// # Why there is no from/to hash
//
// A migration used to declare `from:`/`to:` shape hashes and be treated as an
// EDGE in a hash graph, which meant a file had to correspond to a schema shape
// change: a data-only migration (a backfill, a de-duplication, a correction of
// values an old bug wrote) has the same shape on both sides and was refused at
// parse time. Whether a migration has run is now recorded by NAME in the
// store's [State], the way every comparable tool does it, so a file with no
// shape change is an ordinary migration (TKT-XCJ0Y2).
//
// # Why both projections are still embedded
//
// They are what makes a file self-validating, and neither can be reconstructed
// later. Step targets are checked against the shapes the file itself spans
// (see [Step.Validate]) — `rename_property{from: status, to: state}` is
// well-formed only where `status` exists in the from-shape, and by the time a
// later migration has run, the live schema no longer contains it. And
// [validateDeltasResolved] recomputes the file's OWN edge to refuse a file
// that spans a needs-migration change its steps do not answer; computed
// against the live schema instead, that check would see one aggregate delta
// for the whole chain and could no longer tell which file was meant to resolve
// it — which is precisely how BUG-TMGWIN shipped.
type File struct {
	// Name is the file's base name (e.g. "20260919143022-rename-status.yaml"),
	// validated by [ParseMigrationName]. Files run in lexicographic Name
	// order, which for timestamp-prefixed names is chronological order; the
	// [State] applied list records Names.
	Name string

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
	FromProjection map[string]any `yaml:"from_projection"`
	ToProjection   map[string]any `yaml:"to_projection"`
	Description    string         `yaml:"description,omitempty"`
	Steps          []yaml.Node    `yaml:"steps"`
}

// ParseFile parses and validates one migration file. Every error names the
// file; step errors name the step index and kind too.
//
// The name is validated against the [MigrationName] allowlist: a file whose
// name reaches here from a directory listing has already been filtered by
// [IsMigrationFileName], but ParseFile is also reached directly, and a name
// becomes an applied-list entry that is later compared against directory
// entries.
func ParseFile(name string, data []byte) (*File, error) {
	if _, err := ParseMigrationName(name); err != nil {
		return nil, err
	}
	var raw fileYAML
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("datamigration: %s: %w", name, err)
	}

	fromProj, err := projectionFromYAML(raw.FromProjection)
	if err != nil {
		return nil, fmt.Errorf("datamigration: %s: from_projection: %w", name, err)
	}
	toProj, err := projectionFromYAML(raw.ToProjection)
	if err != nil {
		return nil, fmt.Errorf("datamigration: %s: to_projection: %w", name, err)
	}
	f := &File{
		Name:           name,
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
	"faces_introduced": {"migrate_face"},

	// A swap has exactly one right answer and it is cheap to state, the same
	// reason the face kinds are enforced: every stored edge points the wrong
	// way, and reversing them is the only thing that can be meant.
	"relation_endpoints_swapped": {"reverse_relation"},

	// Deliberately unresolved (TKT-1YBNQJ). Faced → flat leaves rows at named
	// faces that no longer exist while the type's single state is the zero
	// coordinate none of them occupies. Resolving it means moving rows the
	// other way and deciding which face wins when several hold content — a
	// merge, not a move, with its own semantics.
	"faces_removed": {},

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

// resolvingStep is a [Step] that can DISCHARGE a needs-migration delta.
//
// It reports the delta subject it answers, in the subject's own vocabulary:
// an entity-shaped delta names a bare type ("task"), a relation-shaped one is
// prefixed ("rel:blocks", see CompareShapes). Getting that string wrong is
// invisible — the lookup simply misses and the file is refused for having no
// step — so the method exists to keep the mapping beside the step that knows
// it, rather than in a type switch that has to be extended for each new kind.
//
// This WAS a type switch on *migrateFaceStep keyed by its Entity field, which
// silently could not see any other step kind (TKT-HH7PKJ).
type resolvingStep interface {
	Step
	resolvedSubject() string
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
// file cannot parse, `rela migrate gen` must emit a real `migrate_face` step
// rather than a commented suggestion, so the operator reviews a step that will
// actually run instead of one they can ignore into the old behavior.
//
// Only kinds with a non-empty entry in [resolvingSteps] are enforced. The
// deltas are recomputed from the file's OWN embedded projections, so the check
// needs no live schema and cannot drift from what the file claims to do.
func validateDeltasResolved(name string, f *File) error {
	present := map[string]map[string]bool{}
	for _, s := range f.Steps {
		rs, ok := s.(resolvingStep)
		if !ok {
			continue
		}
		if present[s.Kind()] == nil {
			present[s.Kind()] = map[string]bool{}
		}
		present[s.Kind()][rs.resolvedSubject()] = true
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
// One rule: migrate_face reads a property's values to state which face the
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
// feature, not an ordering check. The map_values case is why migrate_face's
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
		case *migrateFaceStep:
			for _, d := range drops {
				if d.entity == step.Entity && d.property == step.Property {
					return fmt.Errorf(
						"datamigration: %s: step %d (migrate_face) reads %s.%s, but step %d (drop_property) "+
							"already removed it — migrate_face must come first, or it would report on no "+
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

// HasMigrations reports whether the directory holds any migration files.
//
// Deliberately a NAME check, not a parse: it feeds the gate's un-baselined
// decision, and that decision must not depend on whether some unrelated file
// happens to be well-formed. A full parse would let one malformed migration
// answer "I cannot tell" to the question "does this project have migrations at
// all", turning a recoverable file error into a refusal to classify.
//
// Both wiring sites (the CLI and appbuild) call THIS, so the two cannot drift
// — the un-baselined decision is safety-critical and deserves one definition.
func HasMigrations(fsys fs.FS) (bool, error) {
	entries, err := fs.ReadDir(fsys, MigrationsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("datamigration: read %s/: %w", MigrationsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && IsMigrationFileName(e.Name()) {
			return true, nil
		}
	}
	return false, nil
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
		// Only YAML is a candidate: migrations/ also holds the committed
		// applied-list (applied.json), and a future sibling would be skipped
		// here too rather than parsed as a migration.
		if ext := path.Ext(e.Name()); ext != ".yaml" && ext != ".yml" {
			continue
		}
		// A YAML file whose NAME is not a migration name is an ERROR, not
		// something to skip quietly. Skipping would mean a migration the
		// operator wrote never runs and nothing says so — the failure mode
		// this system exists to prevent. ParseFile rejects it by name, which
		// is where the message is; listing it here is what gets it there.
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
