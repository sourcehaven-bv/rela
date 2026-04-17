package model

import "github.com/Sourcehaven-BV/rela/internal/entity"

// EntityFromDomain converts an entity.Entity (store domain type) to a
// model.Entity (legacy type). FilePath is left empty.
func EntityFromDomain(e *entity.Entity) *Entity {
	props := make(map[string]interface{}, len(e.Properties))
	for k, v := range e.Properties {
		props[k] = v
	}
	return &Entity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: props,
		Content:    e.Content,
		ModTime:    e.UpdatedAt,
	}
}

// EntityToDomain converts a model.Entity (legacy type) to an
// entity.Entity (store domain type). FilePath is dropped.
func EntityToDomain(e *Entity) *entity.Entity {
	props := make(map[string]interface{}, len(e.Properties))
	for k, v := range e.Properties {
		props[k] = v
	}
	return &entity.Entity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: props,
		Content:    e.Content,
		UpdatedAt:  e.ModTime,
	}
}

// RelationFromDomain converts an entity.Relation (store domain type) to a
// model.Relation (legacy type). FilePath is left empty.
func RelationFromDomain(r *entity.Relation) *Relation {
	var props map[string]interface{}
	if r.Properties != nil {
		props = make(map[string]interface{}, len(r.Properties))
		for k, v := range r.Properties {
			props[k] = v
		}
	}
	return &Relation{
		From:       r.From,
		Type:       r.Type,
		To:         r.To,
		Properties: props,
		Content:    r.Content,
		ModTime:    r.UpdatedAt,
	}
}

// RelationToDomain converts a model.Relation (legacy type) to an
// entity.Relation (store domain type). FilePath is dropped.
func RelationToDomain(r *Relation) *entity.Relation {
	var props map[string]interface{}
	if r.Properties != nil {
		props = make(map[string]interface{}, len(r.Properties))
		for k, v := range r.Properties {
			props[k] = v
		}
	}
	return &entity.Relation{
		From:       r.From,
		Type:       r.Type,
		To:         r.To,
		Properties: props,
		Content:    r.Content,
		UpdatedAt:  r.ModTime,
	}
}
