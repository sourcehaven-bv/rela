package metamodel

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var conversionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var conversionSourceRefPattern = regexp.MustCompile(`\{\{\s*(?:source|new)\.([A-Za-z0-9_-]+)\s*\}\}`)

// validateConversions checks operator-declared type conversions at schema
// load time. An incomplete map must fail closed: silently omitting a field or
// relation during an identity-preserving conversion can lose user data.
func validateConversions(m *Metamodel) []string {
	names := make([]string, 0, len(m.Conversions))
	for name := range m.Conversions {
		names = append(names, name)
	}
	sort.Strings(names)
	var errs []string
	for _, name := range names {
		d := m.Conversions[name]
		bad := func(format string, args ...any) {
			errs = append(errs, fmt.Sprintf("conversion %q: "+format, append([]any{name}, args...)...))
		}
		if strings.TrimSpace(name) == "" {
			bad("name must not be empty")
			continue
		}
		if !conversionNamePattern.MatchString(name) {
			bad("name must match %s", conversionNamePattern.String())
		}
		from, fok := m.GetEntityDef(d.From)
		to, tok := m.GetEntityDef(d.To)
		if !fok {
			bad("from: entity type %q is not declared", d.From)
		}
		if !tok {
			bad("to: entity type %q is not declared", d.To)
		}
		if d.From == d.To {
			bad("from and to must name different entity types")
		}
		if len(d.Fields) == 0 && d.Content == "" {
			bad("requires an explicit `fields:` or `content:` mapping")
		}
		if tok {
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
				if !exists {
					bad("fields: target type %q has no property %q", d.To, key)
				} else if property.Computed != "" {
					bad("fields: target property %q is computed and cannot be mapped", key)
				} else if property.Type == PropertyTypeExternalRef {
					bad("fields: target property %q is an external reference and cannot be mapped", key)
				}
				if fok {
					for _, match := range conversionSourceRefPattern.FindAllStringSubmatch(d.Fields[key], -1) {
						if _, exists := from.Properties[match[1]]; !exists {
							bad("fields.%s: source type %q has no property %q", key, d.From, match[1])
						}
					}
				}
			}
		}
		if fok && d.Content != "" {
			for _, match := range conversionSourceRefPattern.FindAllStringSubmatch(d.Content, -1) {
				if match[1] != "content" {
					if _, exists := from.Properties[match[1]]; !exists {
						bad("content: source type %q has no property %q", d.From, match[1])
					}
				}
			}
		}
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
			if targetRel == "drop" {
				continue
			}
			rd, exists := m.GetRelationDef(targetRel)
			if !exists {
				bad("relations.%s: target relation %q is not declared (or use `drop`)", sourceRel, targetRel)
				continue
			}
			if tok && fok && containsTypeOrAny(sourceDef.From, d.From) && !containsTypeOrAny(rd.From, d.To) {
				bad("relations.%s: target relation %q cannot originate from %q", sourceRel, targetRel, d.To)
			}
			if tok && fok && containsTypeOrAny(sourceDef.To, d.From) && !containsTypeOrAny(rd.To, d.To) {
				bad("relations.%s: target relation %q cannot target %q", sourceRel, targetRel, d.To)
			}
		}
		if fok {
			for relationName, relationDef := range m.Relations {
				applies := containsTypeOrAny(relationDef.From, d.From) || containsTypeOrAny(relationDef.To, d.From)
				if applies {
					if _, mapped := d.Relations[relationName]; !mapped {
						bad("relations: source relation %q needs an explicit mapping or `drop`", relationName)
					}
				}
			}
		}
		if fok && tok {
			for sourceFace, targetFaces := range d.Faces {
				if len(from.Faces) == 0 && sourceFace != "" {
					bad("faces: source type %q has no face %q", d.From, sourceFace)
				}
				if len(from.Faces) > 0 {
					if _, exists := from.Faces[sourceFace]; !exists {
						bad("faces: source type %q has no face %q", d.From, sourceFace)
					}
				}
				if len(targetFaces) == 0 {
					bad("faces.%s: must map to at least one target face", sourceFace)
				}
				for _, targetFace := range targetFaces {
					if targetFace != "" {
						if _, exists := to.Faces[targetFace]; !exists {
							bad("faces: target type %q has no face %q", d.To, targetFace)
						}
					} else if len(to.Faces) > 0 {
						bad("faces: target type %q requires a named face", d.To)
					}
				}
			}
			if len(from.Faces) > 0 {
				for face := range from.Faces {
					if _, exists := d.Faces[face]; !exists {
						bad("faces: source face %q needs an explicit mapping", face)
					}
				}
			}
			if len(from.Faces) == 0 {
				if _, exists := d.Faces[""]; !exists {
					bad("faces: implicit source face needs an explicit mapping")
				}
			}
			if len(to.Faces) > 0 && len(from.Faces) == 0 && len(d.Faces) == 0 {
				bad("faces: converting a faceless source to faced type %q requires an explicit mapping", d.To)
			}
		}
	}
	return errs
}

func containsType(types []string, name string) bool {
	for _, typ := range types {
		if typ == name {
			return true
		}
	}
	return false
}

func containsTypeOrAny(types []string, name string) bool {
	return len(types) == 0 || containsType(types, name)
}
