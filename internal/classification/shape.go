package classification

// BodyField is the pseudo-field for an entity's or relation's markdown body.
const BodyField = "body"

// Shape describes the project schema to this package. The CLI builds it from
// the metamodel, so classification needs no schema dependency.
type Shape struct {
	Entities  map[string]TypeShape
	Relations map[string]RelationShape
}

// TypeShape describes one entity type.
type TypeShape struct {
	// Fields are the classifiable fields in schema order: declared
	// properties, then BodyField (unless a property is already named body).
	Fields []string
	// DisplayFields are the properties the display title is built from.
	// Empty means the title falls back to the entity id.
	DisplayFields []string
	// OpaqueID is false when ids may carry meaning (manual ids, slugs).
	OpaqueID bool
}

// RelationShape describes one relation type.
type RelationShape struct {
	// Fields are the relation's properties (sorted, the schema keeps no
	// order), then BodyField when the relation has content.
	Fields []string
	// Ends lists the entity types the relation connects, with aliases
	// resolved.
	Ends []Ends
}

// Ends is one (from, to) pair a relation type allows.
type Ends struct {
	From, To string
}
