package affordances

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// blockKey is one parsed key of a role's affordance block: the key as
// written, the entity type it names, and the face a `type@face` key names.
type blockKey struct {
	key        string
	entityType string
	face       entity.Face
}

// grantKey returns the key role's compiled grants for bk are stored under.
func (bk blockKey) grantKey(role string) grantKey {
	return grantKey{role: role, entityType: bk.entityType, face: bk.face}
}

// parseBlockKey splits a block key into its type and face. A key without
// `@` names the type and applies on every face. A `type@face` key is
// accepted when faced is true (the `fields:` and `options:` blocks) and must
// name a face the type declares. The type of a bare key is checked per grant
// (validateField), as before.
//
// `visible:` and `relations:` refuse a face key. A `visible:` grant is
// applied inside the search query (SearchFieldScope), which compiles grants
// per type; a face key there would need that lowering to become face-aware.
func (r *PolicyResolver) parseBlockKey(roleName, block, key string, faced bool) (blockKey, error) {
	typeName, faceName, found := strings.Cut(key, entity.StateRefSeparator)
	if !found {
		return blockKey{key: key, entityType: key}, nil
	}
	if !faced {
		return blockKey{}, fmt.Errorf("roles.%s.%s.%s: a %s: block is keyed by entity type only; "+
			"only fields: and options: blocks accept a type@face key", roleName, block, key, block)
	}
	if strings.Contains(faceName, entity.StateRefSeparator) {
		return blockKey{}, fmt.Errorf("roles.%s.%s.%s: more than one %q; a face cannot have a face",
			roleName, block, key, entity.StateRefSeparator)
	}
	def, ok := r.meta.Entities[typeName]
	if !ok {
		return blockKey{}, fmt.Errorf("roles.%s.%s.%s: unknown entity type %q", roleName, block, key, typeName)
	}
	face, err := entity.ParseFace(faceName)
	if err != nil {
		return blockKey{}, fmt.Errorf("roles.%s.%s.%s: %w", roleName, block, key, err)
	}
	if _, declared := def.Faces[faceName]; !declared {
		if len(def.Faces) == 0 {
			return blockKey{}, fmt.Errorf("roles.%s.%s.%s: type %q declares no faces; use the key %q",
				roleName, block, key, typeName, typeName)
		}
		names := slices.Sorted(maps.Keys(def.Faces))
		return blockKey{}, fmt.Errorf("roles.%s.%s.%s: type %q declares no face %q (declared: %s)",
			roleName, block, key, typeName, faceName, strings.Join(names, ", "))
	}
	return blockKey{key: key, entityType: typeName, face: face}, nil
}

// validateField checks that field is a declared property of the block's
// entity type. A typo'd field name would otherwise allow the bogus name and
// closed-world DENY the real one (the inverse of intent), with no
// startup error — so this fails loudly like a predicate compile error
// (RR-RTJE / S2).
func (r *PolicyResolver) validateField(roleName string, bk blockKey, block string, idx int, field string) error {
	def, ok := r.meta.Entities[bk.entityType]
	if !ok {
		return fmt.Errorf("roles.%s.%s.%s[%d]: unknown entity type %q",
			roleName, block, bk.key, idx, bk.entityType)
	}
	if _, ok := def.Properties[field]; !ok {
		return fmt.Errorf("roles.%s.%s.%s[%d]: unknown field %q on type %q",
			roleName, block, bk.key, idx, field, bk.entityType)
	}
	return nil
}

// validateOption checks that field is an enum-typed property of the block's
// entity type and option is one of its declared values.
func (r *PolicyResolver) validateOption(roleName string, bk blockKey, idx int, field, option string) error {
	entityType := bk.entityType
	def, ok := r.meta.Entities[entityType]
	if !ok {
		return fmt.Errorf("roles.%s.options.%s[%d]: unknown entity type %q",
			roleName, bk.key, idx, entityType)
	}
	prop, ok := def.Properties[field]
	if !ok {
		return fmt.Errorf("roles.%s.options.%s[%d]: unknown field %q on type %q",
			roleName, bk.key, idx, field, entityType)
	}
	values := prop.Values
	if len(values) == 0 {
		if ct, ok := r.meta.Types[prop.Type]; ok {
			values = ct.Values
		}
	}
	if len(values) == 0 {
		return fmt.Errorf("roles.%s.options.%s[%d]: field %q on type %q is not enum-typed",
			roleName, bk.key, idx, field, entityType)
	}
	if slices.Contains(values, option) {
		return nil
	}
	return fmt.Errorf("roles.%s.options.%s[%d]: option %q is not a declared value of field %q",
		roleName, bk.key, idx, option, field)
}

// validateRelation checks that relType is a declared relation type
// valid as an outgoing edge from entityType.
func (r *PolicyResolver) validateRelation(roleName, entityType string, idx int, relType string) error {
	rel, ok := r.meta.Relations[relType]
	if !ok {
		return fmt.Errorf("roles.%s.relations.%s[%d]: unknown relation type %q",
			roleName, entityType, idx, relType)
	}
	// A relation grant on an entity type only gates outgoing edges of
	// that type; require entityType to be a valid source.
	if len(rel.From) > 0 {
		if slices.Contains(rel.From, entityType) {
			return nil
		}
		return fmt.Errorf("roles.%s.relations.%s[%d]: relation %q does not originate from type %q",
			roleName, entityType, idx, relType, entityType)
	}
	return nil
}
