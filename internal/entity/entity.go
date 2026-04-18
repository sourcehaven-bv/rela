// Package entity defines the domain types for rela entities and relations.
//
// These types represent the pure domain model — no storage metadata, no
// filesystem paths, no modification times. Storage-layer concerns live
// in the store package; serialization concerns live in the markdown and
// cache packages.
package entity

import (
	"fmt"
	"time"
)

// Entity represents any architecture entity (requirement, decision, etc.).
type Entity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    string                 `json:"content,omitempty"`
	UpdatedAt  time.Time              `json:"updated_at,omitempty"`
}

// New creates a new entity with the given ID and type.
func New(id, entityType string) *Entity {
	return &Entity{
		ID:         id,
		Type:       entityType,
		Properties: make(map[string]interface{}),
	}
}

// GetString returns a string property value.
func (e *Entity) GetString(key string) string {
	if v, ok := e.Properties[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SetString sets a string property value.
func (e *Entity) SetString(key, value string) {
	if e.Properties == nil {
		e.Properties = make(map[string]interface{})
	}
	e.Properties[key] = value
}

// Title returns the entity's title.
func (e *Entity) Title() string {
	return e.GetString("title")
}

// Status returns the entity's status.
func (e *Entity) Status() string {
	return e.GetString("status")
}

// Description returns the entity's description.
func (e *Entity) Description() string {
	return e.GetString("description")
}

// GetAttribute returns struct fields (id, type) or property map values
// uniformly.
func (e *Entity) GetAttribute(name string) interface{} {
	switch name {
	case "id":
		return e.ID
	case "type":
		return e.Type
	default:
		return e.Properties[name]
	}
}

// GetAttributeString returns the string representation of an attribute.
func (e *Entity) GetAttributeString(name string) string {
	val := e.GetAttribute(name)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// GetAttributeStrings returns a property value coerced to []string.
func (e *Entity) GetAttributeStrings(name string) []string {
	val := e.GetAttribute(name)
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// Clone returns a deep copy of the entity.
func (e *Entity) Clone() *Entity {
	clone := &Entity{
		ID:         e.ID,
		Type:       e.Type,
		Content:    e.Content,
		UpdatedAt:  e.UpdatedAt,
		Properties: make(map[string]interface{}, len(e.Properties)),
	}
	for k, v := range e.Properties {
		clone.Properties[k] = CloneValue(v)
	}
	return clone
}

// CloneValue returns a deep copy of a property value.
func CloneValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp
	case []interface{}:
		cp := make([]interface{}, len(val))
		for i, item := range val {
			cp[i] = CloneValue(item)
		}
		return cp
	case map[string]interface{}:
		cp := make(map[string]interface{}, len(val))
		for k, item := range val {
			cp[k] = CloneValue(item)
		}
		return cp
	default:
		return v
	}
}

// Relation represents a directed relationship between two entities.
type Relation struct {
	From       string                 `json:"from"`
	Type       string                 `json:"relation"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    string                 `json:"content,omitempty"`
	UpdatedAt  time.Time              `json:"updated_at,omitempty"`
}

// NewRelation creates a new relation.
func NewRelation(from, relationType, to string) *Relation {
	return &Relation{
		From: from,
		Type: relationType,
		To:   to,
	}
}

// Key returns a unique key for this relation.
func (r *Relation) Key() string {
	return r.From + "--" + r.Type + "--" + r.To
}

// CloneRelation returns a deep copy of the relation.
func (r *Relation) Clone() *Relation {
	clone := &Relation{
		From:      r.From,
		Type:      r.Type,
		To:        r.To,
		Content:   r.Content,
		UpdatedAt: r.UpdatedAt,
	}
	if r.Properties != nil {
		clone.Properties = make(map[string]interface{}, len(r.Properties))
		for k, v := range r.Properties {
			clone.Properties[k] = CloneValue(v)
		}
	}
	return clone
}
