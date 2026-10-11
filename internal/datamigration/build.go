package datamigration

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Spec is a migration whose steps a program has already decided, as opposed
// to a [Generate] draft an operator still has to review. The in-app Configure
// space builds one from choices the user made on screen (TKT-F5NGMG): a
// property they renamed, where the records holding a removed option go.
//
// It deliberately exposes no step types. [Build] renders the spec to the
// on-disk format and parses it back with [ParseFile], so a spec passes exactly
// the checks a hand-written file does: step validation against the embedded
// shapes, every needs-migration change answered, step order.
type Spec struct {
	Description string
	From, To    metamodel.ShapeProjection
	Steps       []SpecStep
}

// SpecStep is one step; exactly one field is set.
type SpecStep struct {
	RenameProperty *RenamePropertySpec
	MapValues      *MapValuesSpec
	SetDefault     *SetDefaultSpec
	Convert        *ConvertSpec
}

// RenamePropertySpec renames an entity property, keeping its values.
type RenamePropertySpec struct{ Entity, From, To string }

// MapValuesSpec rewrites stored values of one property: old value → new.
type MapValuesSpec struct {
	Entity, Property string
	Mapping          map[string]string
}

// SetDefaultSpec fills a property on records that have no value for it.
type SetDefaultSpec struct{ Entity, Property, Value string }

// ConvertSpec converts a property's stored values to another type.
type ConvertSpec struct{ Entity, Property, ToType string }

// Build renders a spec as a migration file named for now, and refuses one
// that would not parse. A nil draft means the spec has no steps and spans no
// needs-migration change, so there is nothing worth a file.
func Build(spec Spec, now time.Time) (*Draft, error) {
	report := metamodel.CompareShapes(spec.From, spec.To)
	if len(spec.Steps) == 0 && report.Compatible() {
		return nil, nil //nolint:nilnil // nil draft = nothing to migrate (documented)
	}
	if strings.TrimSpace(spec.Description) == "" {
		return nil, errors.New("datamigration: Build: a description is required")
	}

	steps := make([]map[string]any, 0, len(spec.Steps))
	for i, s := range spec.Steps {
		step, err := s.yaml()
		if err != nil {
			return nil, fmt.Errorf("datamigration: Build: step %d: %w", i+1, err)
		}
		steps = append(steps, step)
	}
	body, err := yaml.Marshal(map[string]any{"description": spec.Description, "steps": steps})
	if err != nil {
		return nil, err
	}
	fromYAML, err := marshalProjectionYAML("from_projection", spec.From)
	if err != nil {
		return nil, err
	}
	toYAML, err := marshalProjectionYAML("to_projection", spec.To)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("# Written by the Configure space when the schema change was saved.\n")
	b.Write(body)
	b.WriteString(fromYAML)
	b.WriteString(toYAML)

	migName, err := NewMigrationFileName(now.UTC().Format(stampLayout), spec.Description)
	if err != nil {
		return nil, err
	}
	content := []byte(b.String())
	if _, err := ParseFile(migName.String(), content); err != nil {
		return nil, err
	}
	return &Draft{FileName: migName.String(), Content: content, Report: report}, nil
}

func (s SpecStep) yaml() (map[string]any, error) {
	set := 0
	var out map[string]any
	if r := s.RenameProperty; r != nil {
		set++
		out = map[string]any{"rename_property": map[string]any{"entity": r.Entity, "from": r.From, "to": r.To}}
	}
	if m := s.MapValues; m != nil {
		set++
		out = map[string]any{"map_values": map[string]any{
			"entity": m.Entity, "property": m.Property, "mapping": m.Mapping,
		}}
	}
	if d := s.SetDefault; d != nil {
		set++
		out = map[string]any{"set_default": map[string]any{"entity": d.Entity, "property": d.Property, "value": d.Value}}
	}
	if c := s.Convert; c != nil {
		set++
		out = map[string]any{"convert": map[string]any{"entity": c.Entity, "property": c.Property, "to_type": c.ToType}}
	}
	if set != 1 {
		return nil, fmt.Errorf("exactly one step kind must be set, got %d", set)
	}
	return out, nil
}
