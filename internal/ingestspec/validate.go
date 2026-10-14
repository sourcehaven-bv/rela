package ingestspec

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Validate checks a mapping against the metamodel: the find/create
// combination, the entity types and properties they name, find.face, and each
// then: step. Each problem is one message without a subject; the caller
// prefixes the config path that names the mapping, so `"has neither find nor
// create_if_missing"` reads as `webhooks: "x" has neither ...`.
//
// Every problem is meant as a LOAD ERROR. A mapping that silently does nothing,
// or writes to the wrong type, loses records whose producer generally does not
// resend them; failing the load is the safe direction.
//
// Nil: accepted for meta. Without a metamodel only the shape is checked, so
// config-only tests need not build one; every type then reads as unknown.
func (m Mapping) Validate(meta *metamodel.Metamodel) []string {
	var errs []string
	if m.Find == nil && m.CreateIfMissing == nil {
		errs = append(errs, "has neither find nor create_if_missing (nothing to do)")
	}
	if m.Find != nil {
		errs = append(errs, m.validateFind(meta)...)
	}
	if m.CreateIfMissing != nil {
		errs = append(errs, m.validateCreate(meta)...)
	}
	return append(errs, m.validateSteps(meta)...)
}

func (m Mapping) validateCreate(meta *metamodel.Metamodel) []string {
	createType := m.CreateType()
	if createType == "" {
		return []string{"create_if_missing needs a type (set create_if_missing.type or find.type)"}
	}
	def, ok := entityDef(meta, createType)
	if !ok {
		return []string{fmt.Sprintf("create_if_missing type %q is not a known entity type", createType)}
	}
	var errs []string
	for _, prop := range sortedKeys(m.CreateIfMissing.Properties) {
		if _, known := def.Properties[prop]; !known {
			errs = append(errs, fmt.Sprintf(
				"create_if_missing sets unknown property %q on type %q", prop, createType))
		}
	}
	return errs
}

func (m Mapping) validateFind(meta *metamodel.Metamodel) []string {
	find := m.Find
	if find.Type == "" {
		return []string{"find requires a type"}
	}
	def, ok := entityDef(meta, find.Type)
	if !ok {
		return []string{fmt.Sprintf("find type %q is not a known entity type", find.Type)}
	}

	errs := m.validateFace(meta, def)

	// A match key is required only when a create can happen. Without one,
	// find-or-create would create for every record: the find could not
	// identify the entity the previous record made, so each one would mint a
	// duplicate rather than accumulate, which is silent and only visible as
	// growth.
	if len(find.Match) == 0 && m.CreateIfMissing != nil {
		errs = append(errs, "has create_if_missing but find declares no match properties "+
			"(a match key is required to avoid creating a duplicate per record)")
	}

	for _, prop := range find.Match {
		if _, known := def.Properties[prop]; !known {
			errs = append(errs, fmt.Sprintf("find matches on unknown property %q of type %q", prop, find.Type))
		}
	}
	for _, prop := range sortedKeys(find.Values) {
		if !slices.Contains(find.Match, prop) {
			errs = append(errs, fmt.Sprintf("find.values names %q which is not in find.match", prop))
		}
	}
	return errs
}

// validateFace checks find.face against the find type's faces. A create of
// another type has no face to write at, so it must not be faced.
func (m Mapping) validateFace(meta *metamodel.Metamodel, def *metamodel.EntityDef) []string {
	find := m.Find
	var errs []string
	switch {
	case len(def.Faces) == 0 && find.Face != "":
		errs = append(errs, fmt.Sprintf("find.face %q: type %q declares no faces", find.Face, find.Type))
	case len(def.Faces) > 0 && find.Face == "":
		errs = append(errs, fmt.Sprintf("find type %q declares faces, so find.face is required: one of %s",
			find.Type, strings.Join(metamodel.FaceOrderOf(meta, find.Type), ", ")))
	case len(def.Faces) > 0:
		if _, declared := def.Faces[find.Face]; !declared {
			errs = append(errs, fmt.Sprintf("find.face %q is not a face of %q: one of %s",
				find.Face, find.Type, strings.Join(metamodel.FaceOrderOf(meta, find.Type), ", ")))
		}
	}
	if ct := m.CreateType(); ct != "" && ct != find.Type {
		if cdef, ok := entityDef(meta, ct); ok && len(cdef.Faces) > 0 {
			errs = append(errs, fmt.Sprintf("create type %q declares faces; a mapping creates at find.face, "+
				"so the create type must be the find type", ct))
		}
	}
	return errs
}

// validateSteps checks each then: step is a well-formed tagged union.
//
// A step runs against whichever entity the pipeline ended up with: the FOUND
// one, or the one it CREATED. Those can be different types (a mapping may find
// `alpha` and create `beta`), so a step must be valid on every type it could
// reach. Checking only one is wrong in both directions: it rejects a
// legitimate mapping whose step targets the created type, and admits a broken
// one that then fails at run time.
func (m Mapping) validateSteps(meta *metamodel.Metamodel) []string {
	var errs []string
	stepTypes := m.stepTypes()
	for i, step := range m.Then {
		set := 0
		if step.AppendSection != nil {
			set++
		}
		if len(step.Set) > 0 {
			set++
		}
		switch {
		case set == 0:
			errs = append(errs, fmt.Sprintf("then[%d] declares no action (expected append_section or set)", i))
			continue
		case set > 1:
			errs = append(errs, fmt.Sprintf("then[%d] declares more than one action (expected exactly one)", i))
			continue
		}

		if as := step.AppendSection; as != nil && strings.TrimSpace(as.Content) == "" {
			errs = append(errs, fmt.Sprintf("then[%d] append_section requires content", i))
		}

		for _, stepType := range stepTypes {
			def, ok := entityDef(meta, stepType)
			if len(step.Set) == 0 || !ok {
				continue
			}
			for _, prop := range sortedKeys(step.Set) {
				if _, known := def.Properties[prop]; !known {
					errs = append(errs, fmt.Sprintf(
						"then[%d] sets unknown property %q on type %q", i, prop, stepType))
				}
			}
		}
	}
	return errs
}

// stepTypes returns every entity type a then: step could run against,
// deduplicated: the found type and the created type.
func (m Mapping) stepTypes() []string {
	var types []string
	add := func(t string) {
		if t != "" && !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	if m.Find != nil {
		add(m.Find.Type)
	}
	add(m.CreateType())
	return types
}

// Templates calls visit with every interpolated template in the mapping and
// the config path that holds it, so a caller can check the references a
// template uses against the ones its source can resolve.
func (m Mapping) Templates(visit func(path, tmpl string)) {
	if m.Find != nil {
		for _, prop := range sortedKeys(m.Find.Values) {
			visit("find.values."+prop, m.Find.Values[prop])
		}
	}
	if c := m.CreateIfMissing; c != nil {
		for _, prop := range sortedKeys(c.Properties) {
			visit("create_if_missing.properties."+prop, c.Properties[prop])
		}
		visit("create_if_missing.content", c.Content)
	}
	for i, step := range m.Then {
		if as := step.AppendSection; as != nil {
			visit(fmt.Sprintf("then[%d].append_section.content", i), as.Content)
		}
		for _, prop := range sortedKeys(step.Set) {
			visit(fmt.Sprintf("then[%d].set.%s", i, prop), step.Set[prop])
		}
	}
}

// entityDef looks up an entity definition, tolerating a nil metamodel.
func entityDef(meta *metamodel.Metamodel, typeName string) (*metamodel.EntityDef, bool) {
	if meta == nil {
		return nil, false
	}
	return meta.GetEntityDef(typeName)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
