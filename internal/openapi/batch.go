package openapi

const batchTag = "Batch"

// batchEndSchema is one end of a batch relation: a ref to an entity of the
// same batch, or the id of an existing entity.
func batchEndSchema(withFace bool) *Schema {
	props := map[string]*Schema{
		"ref": {Type: "string", Description: "The ref of an entity of this batch"},
		"id":  {Type: "string", Description: "The id of an existing entity"},
	}
	if withFace {
		props["face"] = &Schema{Type: "string", Description: "The face of an existing source, for a content-scoped relation"}
	}
	s := ObjectSchema(props, nil)
	s.Description = "Exactly one of ref and id"
	return s
}

// addBatchRoute adds POST /api/v1/_batch (TKT-BO8CYG9).
func (g *Generator) addBatchRoute(spec *Spec) {
	entity := ObjectSchema(map[string]*Schema{
		"ref":        {Type: "string", Description: "A name for this entity within the batch; unique"},
		"type":       StringSchema(),
		"id":         {Type: "string", Description: "Only for a type with manual ids"},
		"prefix":     {Type: "string", Description: "One of the type's id prefixes"},
		"face":       StringSchema(),
		"properties": {Type: "object", AdditionalProperties: &Schema{}},
		"content":    StringSchema(),
	}, []string{"type"})
	relation := ObjectSchema(map[string]*Schema{
		"from": batchEndSchema(true),
		"type": StringSchema(),
		"to":   batchEndSchema(false),
	}, []string{"from", "type", "to"})
	created := ObjectSchema(map[string]*Schema{
		"index":           IntegerSchema(),
		"ref":             StringSchema(),
		"id":              StringSchema(),
		"face":            StringSchema(),
		"entity":          {Type: "object", Description: "The created entity; absent when you may not read it"},
		"warnings":        ArraySchema(&Schema{Type: "object"}),
		"follow_up_error": {Type: "string", Description: "Set when an automation after the commit failed"},
	}, []string{"index", "id"})
	createdRel := ObjectSchema(map[string]*Schema{
		"index": IntegerSchema(), "from": StringSchema(), "from_face": StringSchema(),
		"type": StringSchema(), "to": StringSchema(),
	}, []string{"index", "from", "type", "to"})

	spec.Paths["/api/v1/_batch"] = PathItem{
		Post: &Operation{
			OperationID: "createBatch",
			Summary:     "Create several entities and the relations between them, atomically",
			Description: "Writes every entity and relation, or none. At most 100 operations and 1 MiB. " +
				"Automations run after the commit and are not part of it. Answers 501 when the " +
				"deployment's store cannot roll back; see capabilities.atomic_batch on /api/v1/_schema. " +
				"An error is a problem whose operation {kind, index, ref} names the operation it is about; " +
				"a 403 adds rule_kind and rule_id.",
			Tags: []string{batchTag},
			RequestBody: &RequestBody{
				Required: true,
				Content: jsonContent(ObjectSchema(map[string]*Schema{
					"entities":  ArraySchema(entity),
					"relations": ArraySchema(relation),
				}, nil)),
			},
			Responses: map[string]Response{
				"200": {
					Description: "Everything was written",
					Content: jsonContent(ObjectSchema(map[string]*Schema{
						"entities":  ArraySchema(created),
						"relations": ArraySchema(createdRel),
					}, []string{"entities", "relations"})),
				},
				"400": {Description: "Invalid JSON, an unknown field, a bad ref, or too many operations",
					Content: problemContent()},
				"403": {Description: "An operation is not allowed", Content: problemContent()},
				"409": {Description: "An id is already taken, or the batch creates the same relation twice",
					Content: problemContent()},
				"413": {Description: "Request body too large", Content: problemContent()},
				"422": {Description: "An operation is invalid, or a relation end does not exist",
					Content: problemContent()},
				"501": {Description: "This deployment cannot write a batch atomically", Content: problemContent()},
			},
		},
	}
}
