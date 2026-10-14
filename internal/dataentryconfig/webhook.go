package dataentryconfig

import (
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
)

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
// # Three workflows
//
// Which of always-create, find-or-create and find-and-update-only a hook runs
// follows from which of Find and CreateIfMissing are set; see the ingestspec
// package doc.
type Webhook struct {
	// Description is operator documentation, unused by the runtime.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Find, CreateIfMissing and Then are the hook's mapping; see
	// [ingestspec.Mapping], which documents each field.
	Find            *WebhookFind   `yaml:"find,omitempty" json:"find,omitempty"`
	CreateIfMissing *WebhookCreate `yaml:"create_if_missing,omitempty" json:"create_if_missing,omitempty"`
	Then            []WebhookStep  `yaml:"then,omitempty" json:"then,omitempty"`

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

// The mapping types live in ingestspec, which the feed subscriptions share;
// these aliases keep the webhook names its callers and tests use.
type (
	WebhookFind          = ingestspec.Find
	WebhookCreate        = ingestspec.Create
	WebhookStep          = ingestspec.Step
	WebhookAppendSection = ingestspec.AppendSection
)

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

// Mapping returns the hook's find / create / then pipeline.
func (w Webhook) Mapping() ingestspec.Mapping {
	return ingestspec.Mapping{Find: w.Find, CreateIfMissing: w.CreateIfMissing, Then: w.Then}
}

// CreateType returns the entity type a create step produces: the explicit
// create type, else the find type.
func (w Webhook) CreateType() string {
	return w.Mapping().CreateType()
}
