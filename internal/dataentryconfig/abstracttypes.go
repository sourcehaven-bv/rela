package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// expandAbstractTypes rewrites the settings that name several entity types,
// so that an abstract schema type stands for all of its variants. An
// abstract type stores no entities, so without this a `create: [task]` or
// an `available_on: {entity_types: [task]}` would offer nothing.
//
// Lists expand in place. Type-keyed maps (`entity_views`, a section's
// `fields_by_type` and the other per-type column maps) copy the abstract
// entry to each variant that has no entry of its own: those entries are a
// complete definition for one type, so an entry naming the variant wins.
//
// Settings that name one type (a form's or a list's `entity_type`) are not
// expanded; the validators report an abstract name there with its variants.
func expandAbstractTypes(cfg *Config, meta *metamodel.Metamodel) {
	if meta == nil || len(metamodel.AbstractTypes(meta)) == 0 {
		return
	}
	names := func(in []string) []string { return expandTypeList(meta, in) }
	for id, a := range cfg.Actions {
		if a.AvailableOn != nil {
			scope := *a.AvailableOn
			scope.EntityTypes = names(scope.EntityTypes)
			a.AvailableOn = &scope
			cfg.Actions[id] = a
		}
	}
	for id, c := range cfg.Commands {
		if c.AvailableOn != nil {
			scope := *c.AvailableOn
			scope.EntityTypes = names(scope.EntityTypes)
			c.AvailableOn = &scope
			cfg.Commands[id] = c
		}
	}
	for i := range cfg.Spaces {
		cfg.Spaces[i].Create = names(cfg.Spaces[i].Create)
	}
	if cfg.Piles != nil {
		for i := range cfg.Piles.Toggles {
			cfg.Piles.Toggles[i].EntityTypes = names(cfg.Piles.Toggles[i].EntityTypes)
		}
	}
	cfg.EntityViews = expandTypeKeys(meta, cfg.EntityViews)
	for id, v := range cfg.Views {
		v.Sections = expandSections(meta, v.Sections)
		cfg.Views[id] = v
	}
	for id, f := range cfg.Forms {
		if f.SidePanel != nil {
			panel := *f.SidePanel
			panel.Sections = expandSections(meta, panel.Sections)
			f.SidePanel = &panel
			cfg.Forms[id] = f
		}
	}
}

func expandSections(meta *metamodel.Metamodel, sections []ViewSection) []ViewSection {
	for i := range sections {
		s := &sections[i]
		s.FieldsByType = expandTypeKeys(meta, s.FieldsByType)
		s.ColumnsByType = expandTypeKeys(meta, s.ColumnsByType)
		s.ParentColumns = expandTypeKeys(meta, s.ParentColumns)
		s.ChildColumns = expandTypeKeys(meta, s.ChildColumns)
	}
	return sections
}

// expandTypeList replaces each abstract name by its variants, dropping
// duplicates and keeping nil as nil.
func expandTypeList(meta *metamodel.Metamodel, in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	add := func(name string) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, name := range in {
		variants, ok := metamodel.VariantsOf(meta, name)
		if !ok {
			add(name)
			continue
		}
		for _, v := range variants {
			add(v)
		}
	}
	return out
}

// expandTypeKeys copies each abstract key's value to every variant without
// a key of its own, and removes the abstract key.
func expandTypeKeys[V any](meta *metamodel.Metamodel, in map[string]V) map[string]V {
	abstract := false
	for key := range in {
		if _, ok := metamodel.VariantsOf(meta, key); ok {
			abstract = true
			break
		}
	}
	if !abstract {
		return in
	}
	out := make(map[string]V, len(in))
	for _, key := range slices.Sorted(maps.Keys(in)) {
		variants, ok := metamodel.VariantsOf(meta, key)
		if !ok {
			out[key] = in[key]
			continue
		}
		for _, v := range variants {
			if _, own := in[v]; !own {
				out[v] = in[key]
			}
		}
	}
	return out
}

// explainAbstractTypes adds the variants to an "unknown entity type" error
// that names an abstract type. A setting that names one type (a form, a
// list, a kanban) cannot stand for several, so the abstract name stays an
// error, but the operator needs to know which names would work.
func explainAbstractTypes(errs []string, meta *metamodel.Metamodel) []string {
	abstracts := metamodel.AbstractTypes(meta)
	if len(abstracts) == 0 {
		return errs
	}
	for i, msg := range errs {
		for _, a := range abstracts {
			if strings.Contains(msg, fmt.Sprintf("unknown entity type %q", a.Name)) {
				errs[i] = fmt.Sprintf("%s (%q is abstract; use one of its variants: %s)",
					msg, a.Name, strings.Join(a.Variants, ", "))
				break
			}
		}
	}
	return errs
}
