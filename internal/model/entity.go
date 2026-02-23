package model

import (
	"fmt"
	"time"
)

// Entity represents any architecture entity (requirement, decision, etc.)
type Entity struct {
	ID         string                 `json:"id" yaml:"id"`
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties" yaml:"properties,omitempty"`
	Content    string                 `json:"content,omitempty" yaml:"-"`
	FilePath   string                 `json:"filePath,omitempty" yaml:"-"`
	ModTime    time.Time              `json:"modTime,omitempty" yaml:"-"`
}

// NewEntity creates a new entity with the given ID and type
func NewEntity(id, entityType string) *Entity {
	return &Entity{
		ID:         id,
		Type:       entityType,
		Properties: make(map[string]interface{}),
	}
}

// GetString returns a string property value
func (e *Entity) GetString(key string) string {
	if v, ok := e.Properties[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SetString sets a string property value
func (e *Entity) SetString(key, value string) {
	if e.Properties == nil {
		e.Properties = make(map[string]interface{})
	}
	e.Properties[key] = value
}

// Title returns the entity's title
func (e *Entity) Title() string {
	return e.GetString("title")
}

// Status returns the entity's status
func (e *Entity) Status() Status {
	return Status(e.GetString("status"))
}

// Description returns the entity's description
func (e *Entity) Description() string {
	return e.GetString("description")
}

// GetAttribute returns struct fields (id, type) or property map values uniformly.
// This allows renderers to access both built-in fields and custom properties
// without special-case handling.
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
// Returns empty string for nil values.
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
// Handles []string, []interface{} (extracting string elements), and nil.
// Returns nil for non-list values.
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

// Relation represents a directed relationship between two entities
type Relation struct {
	From       string                 `json:"from" yaml:"from"`
	Type       string                 `json:"relation" yaml:"relation"`
	To         string                 `json:"to" yaml:"to"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Content    string                 `json:"content,omitempty" yaml:"-"`
	FilePath   string                 `json:"filePath,omitempty" yaml:"-"`
	ModTime    time.Time              `json:"modTime,omitempty" yaml:"-"`
}

// NewRelation creates a new relation
func NewRelation(from, relationType, to string) *Relation {
	return &Relation{
		From: from,
		Type: relationType,
		To:   to,
	}
}

// Key returns a unique key for this relation
func (r *Relation) Key() string {
	return r.From + "--" + r.Type + "--" + r.To
}

// Clone returns a deep copy of the entity.
func (e *Entity) Clone() *Entity {
	clone := &Entity{
		ID:         e.ID,
		Type:       e.Type,
		Content:    e.Content,
		FilePath:   e.FilePath,
		ModTime:    e.ModTime,
		Properties: make(map[string]interface{}, len(e.Properties)),
	}
	for k, v := range e.Properties {
		clone.Properties[k] = v
	}
	return clone
}
