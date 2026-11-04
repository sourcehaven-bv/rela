package v1

// BatchRequest is the body of POST /api/v1/_batch: entities to create and
// the relations between them and existing entities, written all or nothing.
type BatchRequest struct {
	Entities  []BatchEntity   `json:"entities"`
	Relations []BatchRelation `json:"relations"`
}

// BatchEntity is one entity create of a [BatchRequest]. Ref is a label a
// relation of the same request uses to name this entity; it is never
// stored. ID and Prefix are those of a single create: an id the caller
// chooses, where the type admits one, or a prefix for a generated id.
type BatchEntity struct {
	Ref        string         `json:"ref,omitempty"`
	Type       string         `json:"type"`
	ID         string         `json:"id,omitempty"`
	Prefix     string         `json:"prefix,omitempty"`
	Face       string         `json:"face,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// BatchRelation is one relation create of a [BatchRequest].
type BatchRelation struct {
	From BatchEnd `json:"from"`
	Type string   `json:"type"`
	To   BatchEnd `json:"to"`
}

// BatchEnd names one end of a [BatchRelation]: Ref for an entity of the
// same request, or ID for an existing entity. Exactly one is set. Face is
// the tail of an edge from an existing entity; an edge from a batch entity
// leaves that entity's face, and a head never names a face.
type BatchEnd struct {
	Ref  string `json:"ref,omitempty"`
	ID   string `json:"id,omitempty"`
	Face string `json:"face,omitempty"`
}

// BatchResponse is the 200 answer of POST /api/v1/_batch, in request order.
type BatchResponse struct {
	Entities  []BatchCreatedEntity   `json:"entities"`
	Relations []BatchCreatedRelation `json:"relations"`
}

// BatchCreatedEntity is one created entity. Entity is the row as the
// caller may read it: property values the caller may not see are left
// out, and Entity is absent for a row the caller cannot read at all.
// FollowUpError is set when the on-create automations or cascade failed
// after the commit; the entity stays created.
type BatchCreatedEntity struct {
	Index         int       `json:"index"`
	Ref           string    `json:"ref,omitempty"`
	ID            string    `json:"id"`
	Face          string    `json:"face,omitempty"`
	Entity        *Entity   `json:"entity,omitempty"`
	Warnings      []Warning `json:"warnings,omitempty"`
	FollowUpError string    `json:"follow_up_error,omitempty"`
}

// BatchCreatedRelation is one created relation.
type BatchCreatedRelation struct {
	Index    int    `json:"index"`
	From     string `json:"from"`
	FromFace string `json:"from_face,omitempty"`
	Type     string `json:"type"`
	To       string `json:"to"`
}

// ErrorOperation names the operation of a batch request an [Error] is
// about: Kind is "entity" or "relation", Index its position in that list,
// and Ref the entity's ref when it has one.
type ErrorOperation struct {
	Kind  string `json:"kind"`
	Index int    `json:"index"`
	Ref   string `json:"ref,omitempty"`
}
