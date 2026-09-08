package dataentryconfig

import "net/http"

// Webhook declares an inbound HTTP endpoint that maps a request onto entity
// writes, without Lua (TKT-1EM4KL).
//
// # The hook id IS the URL segment
//
// A hook keyed `icinga-alert` serves POST /hooks/icinga-alert. There is
// deliberately no `path:` key: a hook needs an id in the YAML regardless, so an
// explicit path would add an aliasing surface to design, validate and keep
// consistent with the id, for no gain.
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
type Webhook struct {
	// Description is operator documentation, unused by the runtime.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Find locates an existing entity to act on. Omitted ⇒ always create.
	Find *WebhookFind `yaml:"find,omitempty" json:"find,omitempty"`

	// CreateIfMissing creates the entity when Find matches nothing (or when
	// Find is omitted entirely). Nil ⇒ a miss is a no-op, reported as
	// no_match in the response.
	CreateIfMissing *WebhookCreate `yaml:"create_if_missing,omitempty" json:"create_if_missing,omitempty"`

	// Then are the mutation steps applied to the found-or-created entity, in
	// order. Steps must be PURE: a conflict retry re-runs the whole list, so a
	// step with an external side effect would repeat it.
	Then []WebhookStep `yaml:"then,omitempty" json:"then,omitempty"`

	// Respond shapes the HTTP response. Zero value ⇒ 200.
	Respond WebhookRespond `yaml:"respond,omitempty" json:"respond,omitzero"`

	// Headers is an ALLOWLIST of request header names exposed to templates as
	// {{header.<name>}}. Empty ⇒ no headers are exposed.
	//
	// An allowlist rather than pass-through is a security requirement, not a
	// convenience: headers carry session cookies, bearer tokens and
	// proxy-injected identity assertions, any of which would otherwise be
	// interpolatable into stored entity content by an operator who only wanted
	// a delivery id.
	Headers []string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// MaxBodyBytes overrides the default inbound body cap for this hook.
	// Zero ⇒ DefaultWebhookMaxBodyBytes.
	MaxBodyBytes int64 `yaml:"max_body_bytes,omitempty" json:"max_body_bytes,omitempty"`
}

// WebhookFind locates the entity a delivery concerns.
type WebhookFind struct {
	// Type is the entity type to search. Required.
	Type string `yaml:"type" json:"type"`

	// Match names the properties whose values identify the entity. Each is
	// compared against the interpolated value in Values, or — when Values has
	// no entry for it — against the same-named field of the request body.
	//
	// It names SOURCE properties, never a derived key: computed properties are
	// materialized on write, so a `unique:` computed hash exists on the stored
	// entity and matching on its inputs is what lets rela derive the identity.
	Match []string `yaml:"match,omitempty" json:"match,omitempty"`

	// Values optionally supplies an interpolated expression per match property,
	// for when the body's field name differs from the entity's property name.
	Values map[string]string `yaml:"values,omitempty" json:"values,omitempty"`
}

// WebhookCreate describes the entity to create on a miss.
type WebhookCreate struct {
	// Type is the entity type to create. Defaults to Find.Type when omitted.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Template names a template variant (templates/entities/<type>--<variant>.md).
	Template string `yaml:"template,omitempty" json:"template,omitempty"`

	// Properties are interpolated property values for the new entity.
	Properties map[string]string `yaml:"properties,omitempty" json:"properties,omitempty"`

	// Content is the interpolated markdown body of the new entity.
	Content string `yaml:"content,omitempty" json:"content,omitempty"`
}

// WebhookStep is one mutation applied to the found-or-created entity. Exactly
// one field must be set — the shape is a tagged union spelled as a struct so
// YAML reads naturally.
type WebhookStep struct {
	// AppendSection appends a line to a named markdown section of the body.
	AppendSection *WebhookAppendSection `yaml:"append_section,omitempty" json:"append_section,omitempty"`

	// Set upserts interpolated property values.
	Set map[string]string `yaml:"set,omitempty" json:"set,omitempty"`
}

// WebhookAppendSection appends a line to a named section of the entity body.
//
// A missing section is CREATED rather than erroring — see
// markdown.AppendToSection, which owns that decision and the reasoning.
type WebhookAppendSection struct {
	// Section is the heading text to append under. Matched case-insensitively.
	Section string `yaml:"section" json:"section"`

	// Content is the interpolated line to append.
	Content string `yaml:"content" json:"content"`
}

// WebhookRespond shapes the response to a successful delivery.
type WebhookRespond struct {
	// Status is the HTTP status returned on success. Zero ⇒ 200.
	Status int `yaml:"status,omitempty" json:"status,omitempty"`
}

// StatusOrDefault returns the configured success status, or 200.
func (r WebhookRespond) StatusOrDefault() int {
	if r.Status == 0 {
		return http.StatusOK
	}
	return r.Status
}

// CreateType returns the entity type a create step produces: the explicit
// create type, else the find type.
func (w Webhook) CreateType() string {
	if w.CreateIfMissing == nil {
		return ""
	}
	if w.CreateIfMissing.Type != "" {
		return w.CreateIfMissing.Type
	}
	if w.Find != nil {
		return w.Find.Type
	}
	return ""
}
