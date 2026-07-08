package entity

// Reserved keys name the identity fields of an [Entity] or [Relation] in
// tabular or frontmatter form (markdown files, CSV imports, filter/sort
// targets). They map to struct fields, never to Properties: parsers must
// skip them when building a Properties map, and validators must accept
// them even though they are not schema-declared properties.
//
// Formats may reserve additional keys of their own on top of these (for
// example entity template files reserve a templating-only relations key);
// such keys are policy of the owning package, composed with these
// predicates at the call site.

// IsReservedEntityKey reports whether key names an identity field of an
// [Entity] ("id", "type") rather than a property.
func IsReservedEntityKey(key string) bool {
	return key == "id" || key == "type"
}

// IsReservedRelationKey reports whether key names an identity field of a
// [Relation] ("from", "relation", "to") rather than a property. Note the
// type of a relation is keyed "relation", not "type", in frontmatter.
func IsReservedRelationKey(key string) bool {
	return key == "from" || key == "relation" || key == "to"
}
