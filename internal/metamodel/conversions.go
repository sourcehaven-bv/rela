package metamodel

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var conversionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var conversionSourceRefPattern = regexp.MustCompile(`\{\{\s*(?:source|new)\.([A-Za-z0-9_-]+)\s*\}\}`)

// ConversionDrop is the `relations:` value of a conversion that drops the
// relation instead of mapping it. No relation type may take this name
// while conversions are declared, or a mapping to it would read as a drop.
const ConversionDrop = "drop"

// validateConversions checks operator-declared type conversions at schema
// load time. An incomplete map must fail closed: silently omitting a field or
// relation during an identity-preserving conversion can lose user data.
func validateConversions(m *Metamodel) []string {
	names := make([]string, 0, len(m.Conversions))
	for name := range m.Conversions {
		names = append(names, name)
	}
	sort.Strings(names)
	errs := make([]string, 0, len(m.Conversions))
	if _, clash := m.Relations[ConversionDrop]; clash && len(m.Conversions) > 0 {
		errs = append(errs, fmt.Sprintf("relation type %q is reserved while conversions are declared", ConversionDrop))
	}
	for _, name := range names {
		errs = append(errs, validateConversion(m, name, m.Conversions[name])...)
	}
	return errs
}

type conversionProblem func(string, ...any)

func validateConversion(m *Metamodel, name string, d ConversionDef) []string {
	var errs []string
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf("conversion %q: "+format, append([]any{name}, args...)...))
	}
	if strings.TrimSpace(name) == "" {
		bad("name must not be empty")
		return errs
	}
	if !conversionNamePattern.MatchString(name) {
		bad("name must match %s", conversionNamePattern.String())
	}
	from, fromOK := m.GetEntityDef(d.From)
	to, toOK := m.GetEntityDef(d.To)
	if !fromOK {
		bad("from: entity type %q is not declared", d.From)
	}
	if !toOK {
		bad("to: entity type %q is not declared", d.To)
	}
	if d.From == d.To {
		bad("from and to must name different entity types")
	}
	if len(d.Fields) == 0 && d.Content == "" {
		bad("requires an explicit `fields:` or `content:` mapping")
	}
	if toOK {
		validateConversionFields(d, *from, *to, fromOK, bad)
	}
	if fromOK && d.Content != "" {
		validateConversionContent(d, *from, bad)
	}
	validateConversionRelations(m, d, fromOK, toOK, bad)
	if fromOK && toOK {
		validateConversionFaces(d, *from, *to, bad)
	}
	return errs
}

func validateConversionFields(d ConversionDef, from, to EntityDef, fromOK bool, bad conversionProblem) {
	for key, property := range to.Properties {
		if property.Type == PropertyTypeExternalRef && property.Required {
			bad("target property %q is a required external reference and cannot be authored by conversion", key)
		}
	}
	fields := make([]string, 0, len(d.Fields))
	for key := range d.Fields {
		fields = append(fields, key)
	}
	sort.Strings(fields)
	for _, key := range fields {
		property, exists := to.Properties[key]
		switch {
		case !exists:
			bad("fields: target type %q has no property %q", d.To, key)
		case property.Computed != "":
			bad("fields: target property %q is computed and cannot be mapped", key)
		case property.Type == PropertyTypeExternalRef:
			bad("fields: target property %q is an external reference and cannot be mapped", key)
		}
		if fromOK {
			validateSourceReferences(d.Fields[key], "fields."+key, d, from, bad)
		}
	}
}

func validateConversionContent(d ConversionDef, from EntityDef, bad conversionProblem) {
	for _, match := range conversionSourceRefPattern.FindAllStringSubmatch(d.Content, -1) {
		if match[1] == "content" {
			continue
		}
		if _, exists := from.Properties[match[1]]; !exists {
			bad("content: source type %q has no property %q", d.From, match[1])
		}
	}
}

func validateSourceReferences(template, path string, d ConversionDef, from EntityDef, bad conversionProblem) {
	for _, match := range conversionSourceRefPattern.FindAllStringSubmatch(template, -1) {
		if _, exists := from.Properties[match[1]]; !exists {
			bad("%s: source type %q has no property %q", path, d.From, match[1])
		}
	}
}

func validateConversionRelations(m *Metamodel, d ConversionDef, fromOK, toOK bool, bad conversionProblem) {
	rels := make([]string, 0, len(d.Relations))
	for key := range d.Relations {
		rels = append(rels, key)
	}
	sort.Strings(rels)
	for _, sourceRel := range rels {
		sourceDef, exists := m.GetRelationDef(sourceRel)
		if !exists {
			bad("relations: source relation %q is not declared", sourceRel)
			continue
		}
		targetRel := d.Relations[sourceRel]
		if targetRel == ConversionDrop {
			continue
		}
		targetDef, exists := m.GetRelationDef(targetRel)
		if !exists {
			bad("relations.%s: target relation %q is not declared (or use `drop`)", sourceRel, targetRel)
			continue
		}
		validateConversionRelationEnds(sourceRel, targetRel, d, *sourceDef, *targetDef, fromOK && toOK, bad)
	}
	if fromOK {
		for relationName, relationDef := range m.Relations {
			applies := containsTypeOrAny(relationDef.From, d.From) || containsTypeOrAny(relationDef.To, d.From)
			if applies {
				if _, mapped := d.Relations[relationName]; !mapped {
					bad("relations: source relation %q needs an explicit mapping or `drop`", relationName)
				}
			}
		}
	}
}

func validateConversionRelationEnds(
	sourceName, targetName string,
	d ConversionDef,
	source, target RelationDef,
	typesOK bool,
	bad conversionProblem,
) {
	if !typesOK {
		return
	}
	if containsTypeOrAny(source.From, d.From) && !containsTypeOrAny(target.From, d.To) {
		bad("relations.%s: target relation %q cannot originate from %q", sourceName, targetName, d.To)
	}
	if containsTypeOrAny(source.To, d.From) && !containsTypeOrAny(target.To, d.To) {
		bad("relations.%s: target relation %q cannot target %q", sourceName, targetName, d.To)
	}
}

func validateConversionFaces(d ConversionDef, from, to EntityDef, bad conversionProblem) {
	if len(from.Faces) == 0 && len(to.Faces) == 0 && len(d.Faces) == 0 {
		return
	}
	for sourceFace, targets := range d.Faces {
		validateConversionSourceFace(d, from, sourceFace, bad)
		if len(targets) == 0 {
			bad("faces.%s: must map to at least one target face", sourceFace)
		}
		for _, targetFace := range targets {
			validateConversionTargetFace(d, to, targetFace, bad)
		}
	}
	validateConversionFaceCoverage(d, from, to, bad)
}

func validateConversionSourceFace(d ConversionDef, from EntityDef, face string, bad conversionProblem) {
	if len(from.Faces) == 0 && face != "" {
		bad("faces: source type %q has no face %q", d.From, face)
		return
	}
	if _, exists := from.Faces[face]; !exists && len(from.Faces) > 0 {
		bad("faces: source type %q has no face %q", d.From, face)
	}
}

func validateConversionTargetFace(d ConversionDef, to EntityDef, face string, bad conversionProblem) {
	if face == "" {
		if len(to.Faces) > 0 {
			bad("faces: target type %q requires a named face", d.To)
		}
		return
	}
	if _, exists := to.Faces[face]; !exists {
		bad("faces: target type %q has no face %q", d.To, face)
	}
}

func validateConversionFaceCoverage(d ConversionDef, from, to EntityDef, bad conversionProblem) {
	for face := range from.Faces {
		if _, exists := d.Faces[face]; !exists {
			bad("faces: source face %q needs an explicit mapping", face)
		}
	}
	if len(from.Faces) == 0 && len(to.Faces) > 0 {
		if _, exists := d.Faces[""]; !exists {
			bad("faces: implicit source face needs an explicit mapping to target faces")
		}
	}
}

func containsType(types []string, name string) bool {
	return slices.Contains(types, name)
}

func containsTypeOrAny(types []string, name string) bool {
	return len(types) == 0 || containsType(types, name)
}
