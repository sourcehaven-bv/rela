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
	return f, nil
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
