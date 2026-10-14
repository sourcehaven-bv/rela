// Package ingestspec declares the find / create / then mapping that turns an
// inbound record into entity writes. A declarative webhook (data-entry.yaml
// `webhooks:`) is one source of records; package ingest runs the mapping.
//
// Near-leaf on purpose: config packages embed these types, and must not pull
// in the store and write path that ingest needs to execute them. Its one
// project import is metamodel, which [Mapping.Validate] checks against.
//
// # Three workflows, distinguished by which fields are set
//
// Uniqueness is a property of the operator's schema, not something this layer
// may assume, so all three shapes must be expressible:
//
//	Find    CreateIfMissing   workflow
//	------- ----------------- ---------------------------------------
//	nil     set               always-create (an inbound form post)
//	set     set               find-or-create (a monitoring alert)
//	set     nil               find-and-update-only (no create race)
//
// This is why Find is OPTIONAL and a match key is required only alongside
// CreateIfMissing: requiring a key everywhere would break always-create
// outright.
package ingestspec

// Mapping is one find → create-if-missing → then pipeline.
type Mapping struct {
	// Find locates an existing entity to act on. Omitted ⇒ always create.
	Find *Find `yaml:"find,omitempty" json:"find,omitempty"`

	// CreateIfMissing creates the entity when Find matches nothing (or when
	// Find is omitted entirely). Nil ⇒ a miss is a no-op, reported as
	// no_match.
	CreateIfMissing *Create `yaml:"create_if_missing,omitempty" json:"create_if_missing,omitempty"`

	// Then are the mutation steps applied to the found-or-created entity, in
	// order. Steps must be PURE: a conflict retry re-runs the whole list, so a
	// step with an external side effect would repeat it.
	Then []Step `yaml:"then,omitempty" json:"then,omitempty"`
}

// Find locates the entity a record concerns.
type Find struct {
	// Type is the entity type to search. Required.
	Type string `yaml:"type" json:"type"`

	// Face is the face the mapping reads, writes and creates, for a Type that
	// declares `faces:`. Required for such a type and refused for one
	// without. A record names no face, and a world may rank several, so
	// the face a mapping edits is configuration, never a world's choice
	// (TKT-7IZHP0 §6).
	Face string `yaml:"face,omitempty" json:"face,omitempty"`

	// Match names the properties whose values identify the entity. Each is
	// compared against the interpolated value in Values, or, when Values has
	// no entry for it, against the source's default for that property (the
	// same-named body field, for a webhook).
	//
	// It names SOURCE properties, never a derived key: computed properties are
	// materialized on write, so a `unique:` computed hash exists on the stored
	// entity and matching on its inputs is what lets rela derive the identity.
	Match []string `yaml:"match,omitempty" json:"match,omitempty"`

	// Values optionally supplies an interpolated expression per match property,
	// for when the record's field name differs from the entity's property name.
	Values map[string]string `yaml:"values,omitempty" json:"values,omitempty"`
}

// Create describes the entity to create on a miss.
type Create struct {
	// Type is the entity type to create. Defaults to Find.Type when omitted.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Template names a template variant (templates/entities/<type>--<variant>.md).
	Template string `yaml:"template,omitempty" json:"template,omitempty"`

	// Properties are interpolated property values for the new entity.
	Properties map[string]string `yaml:"properties,omitempty" json:"properties,omitempty"`

	// Content is the interpolated markdown body of the new entity.
	Content string `yaml:"content,omitempty" json:"content,omitempty"`
}

// Step is one mutation applied to the found-or-created entity. Exactly one
// field must be set: the shape is a tagged union spelled as a struct so YAML
// reads naturally.
type Step struct {
	// AppendSection appends a line to a named markdown section of the body.
	AppendSection *AppendSection `yaml:"append_section,omitempty" json:"append_section,omitempty"`

	// Set upserts interpolated property values.
	Set map[string]string `yaml:"set,omitempty" json:"set,omitempty"`
}

// AppendSection appends a line to a named section of the entity body.
//
// A missing section is CREATED rather than erroring; markdown.AppendToSection
// owns that decision and the reasoning.
type AppendSection struct {
	// Section is the heading text to append under. Matched case-insensitively.
	Section string `yaml:"section" json:"section"`

	// Content is the interpolated line to append.
	Content string `yaml:"content" json:"content"`
}

// CreateType returns the entity type a create step produces: the explicit
// create type, else the find type. Empty when the mapping never creates.
func (m Mapping) CreateType() string {
	if m.CreateIfMissing == nil {
		return ""
	}
	if m.CreateIfMissing.Type != "" {
		return m.CreateIfMissing.Type
	}
	if m.Find != nil {
		return m.Find.Type
	}
	return ""
}

// NeedsBody reports whether any step splices the markdown body, and so needs
// a fresh base to compute the replacement from.
func (m Mapping) NeedsBody() bool {
	for _, step := range m.Then {
		if step.AppendSection != nil {
			return true
		}
	}
	return false
}
