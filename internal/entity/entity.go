// Package entity defines the domain types for rela entities and relations.
//
// These types represent the pure domain model — no storage metadata, no
// filesystem paths, no modification times. Storage-layer concerns live
// in the store package; serialization concerns live in the markdown and
// cache packages.
package entity

import (
	"fmt"
	"slices"
	"time"
)

// InaccessibleReason explains why a property's value is unreadable.
//
// Today only [InaccessibleReasonGitCrypt] is produced; the type is an
// enum so future sources of the same condition (e.g. SOPS-style field
// encryption) can extend it without changing the shape.
//
// NOT for ACL field redaction — use [Entity.Redacted]. The two look
// alike and are not: Inaccessible means the STORED BYTES cannot be read
// by anyone here, so [Entity.IsLocked] blocks writes that would replace
// ciphertext with a cleartext shell. ACL redaction means this PRINCIPAL
// may not see a value that is otherwise intact and writable. Recording
// a redaction here would make every gated read look locked — the
// validator silently skips locked entities (internal/validator, which
// reads through a gated reader), and the data-entry write path would
// reject them with a git-crypt error message.
type InaccessibleReason string

const (
	// InaccessibleReasonGitCrypt indicates the file is git-crypt encrypted
	// and the key is not present in the local working tree.
	InaccessibleReasonGitCrypt InaccessibleReason = "git-crypt"
)

// InaccessibleFieldContent is the reserved Name used in
// [InaccessibleField] to mark the entity or relation's markdown body
// as inaccessible (as distinct from any schema-declared property).
const InaccessibleFieldContent = "content"

// InaccessibleField marks a single field as known-to-exist but not
// readable by the holder of the entity. The Name is either a property
// name or the [InaccessibleFieldContent] sentinel naming the markdown
// body. A property name appears in [Entity.Properties] OR in
// [Entity.Inaccessible], never both.
type InaccessibleField struct {
	Name   string             `json:"name"`
	Reason InaccessibleReason `json:"reason"`
}

// Entity represents any architecture entity (requirement, decision, etc.).
type Entity struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	// Face addresses the content state this record is (TKT-DOFYR1).
	// Zero value = the default state / a faceless entity, so a
	// project without faces never sees the field (omitempty). See
	// [Face] for the two load-bearing rules (codec-only construction;
	// stores equality-match, never inspect).
	Face Face `json:"face,omitempty"`

	Properties   map[string]any      `json:"properties,omitempty"`
	Content      string              `json:"content,omitempty"`
	UpdatedAt    time.Time           `json:"updated_at,omitzero"`
	Inaccessible []InaccessibleField `json:"inaccessible,omitempty"`

	// Redacted names the properties withheld from the reading principal
	// by field-level ACL (`visible:`), sorted. Populated by
	// visibility.Redact, the one read-out choke point; empty on every
	// ungated path.
	//
	// A PER-READER ARTIFACT, never content: it is not persisted, and
	// canonical.HashEntity ignores it (like Inaccessible) so two
	// principals compute the same content hash for the same entity.
	//
	// Distinct from Inaccessible — see [InaccessibleReason]. A redacted
	// property is readable by SOMEONE and the entity stays writable, so
	// this deliberately does NOT feed [Entity.IsLocked].
	//
	// Disclosing NAMES is intended: field-level redaction hides property
	// values only, and the metamodel that declares those names is already
	// served over /api/v1/_schema. The HTTP surface ships the same list as
	// `_redacted` (DEC-T0XIWQ); this is the in-process equivalent.
	Redacted []string `json:"redacted,omitempty"`
}

// IsRedacted reports whether the named property was withheld from the
// reading principal by field-level ACL. False on ungated read paths,
// where no policy was evaluated.
func (e *Entity) IsRedacted(name string) bool {
	return slices.Contains(e.Redacted, name)
}

// IsInaccessible reports whether the named property is in [Entity.Inaccessible].
func (e *Entity) IsInaccessible(name string) bool {
	for _, f := range e.Inaccessible {
		if f.Name == name {
			return true
		}
	}
	return false
}

// IsLocked reports whether the entity has any field marked unreadable.
// True for any entity returned from a load that could not produce
// readable property values (today: git-crypt encrypted files; future:
// SOPS field encryption, Lua-driven access control).
//
// Write paths must reject operations on locked entities — the
// underlying file is unreadable, so writing through the in-memory
// representation would replace whatever it contains (typically
// ciphertext) with cleartext form data.
func (e *Entity) IsLocked() bool {
	return len(e.Inaccessible) > 0
}

// New creates a new entity with the given ID and type.
func New(id, entityType string) *Entity {
	return &Entity{
		ID:         id,
		Type:       entityType,
		Properties: make(map[string]any),
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
		e.Properties = make(map[string]any)
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
func (e *Entity) GetAttribute(name string) any {
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
	case []any:
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
		Face:       e.Face,
		Content:    e.Content,
		UpdatedAt:  e.UpdatedAt,
		Properties: make(map[string]any, len(e.Properties)),
	}
	for k, v := range e.Properties {
		clone.Properties[k] = CloneValue(v)
	}
	if len(e.Inaccessible) > 0 {
		clone.Inaccessible = make([]InaccessibleField, len(e.Inaccessible))
		copy(clone.Inaccessible, e.Inaccessible)
	}
	if len(e.Redacted) > 0 {
		clone.Redacted = slices.Clone(e.Redacted)
	}
	return clone
}

// CloneValue returns a deep copy of a property value.
func CloneValue(v any) any {
	switch val := v.(type) {
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = CloneValue(item)
		}
		return cp
	case map[string]any:
		cp := make(map[string]any, len(val))
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
	From string `json:"from"`

	// FromFace is the state-specific TAIL of a content-scoped edge
	// (design doc §2.3): the edge attaches to (From, FromFace). Zero
	// value = the tail is the default state / an identity-scoped edge.
	// Heads are entity-level by construction — there is deliberately NO
	// ToFace, which is what makes cross-world dangling references
	// inexpressible.
	FromFace Face `json:"from_face,omitempty"`

	Type         string              `json:"relation"`
	To           string              `json:"to"`
	Properties   map[string]any      `json:"properties,omitempty"`
	Content      string              `json:"content,omitempty"`
	UpdatedAt    time.Time           `json:"updated_at,omitzero"`
	Inaccessible []InaccessibleField `json:"inaccessible,omitempty"`
}

// IsInaccessible reports whether the named property is in [Relation.Inaccessible].
func (r *Relation) IsInaccessible(name string) bool {
	for _, f := range r.Inaccessible {
		if f.Name == name {
			return true
		}
	}
	return false
}

// IsLocked reports whether the relation has any field marked unreadable.
// See [Entity.IsLocked] for semantics.
func (r *Relation) IsLocked() bool {
	return len(r.Inaccessible) > 0
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
	// The FROM slot carries the tail face via the codec serialization
	// (TKT-DOFYR1) — two edges on the same triple with different tails
	// are two relations, so the face is part of the key. The face
	// grammar forbids "--", keeping the key unambiguous; a default-tail
	// key is byte-identical to the historical form.
	return FormatStateRef(r.From, r.FromFace) + "--" + r.Type + "--" + r.To
}

// CloneRelation returns a deep copy of the relation.
func (r *Relation) Clone() *Relation {
	clone := &Relation{
		From:      r.From,
		FromFace:  r.FromFace,
		Type:      r.Type,
		To:        r.To,
		Content:   r.Content,
		UpdatedAt: r.UpdatedAt,
	}
	if r.Properties != nil {
		clone.Properties = make(map[string]any, len(r.Properties))
		for k, v := range r.Properties {
			clone.Properties[k] = CloneValue(v)
		}
	}
	if len(r.Inaccessible) > 0 {
		clone.Inaccessible = make([]InaccessibleField, len(r.Inaccessible))
		copy(clone.Inaccessible, r.Inaccessible)
	}
	return clone
}
