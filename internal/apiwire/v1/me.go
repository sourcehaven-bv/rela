package v1

// MeResponse is the body of GET /api/v1/_me: who the request principal is,
// for the SPA's account menu (TKT-MJTD12).
//
// It is display data only. Nothing the SPA does with it is an access
// decision; the server keeps authorizing every request on the verified
// principal. Every field but User is omitted when unknown, so a deployment
// with no proxy, or a proxy that sends only `sub`, gets `{"user": "..."}`.
type MeResponse struct {
	// User is the principal's user id: the resolved person entity id when
	// acl.yaml's principal_property matched one, else the raw identity.
	User string `json:"user"`
	// Email is the verified `email` claim.
	Email string `json:"email,omitempty"`
	// Org is the org the session is scoped to, from the verified `org_id`,
	// `org_slug` and `org_name` claims. Absent without an `org_id`.
	Org *MeOrg `json:"org,omitempty"`
	// Roles are the role names the verified assertion's `roles` claim
	// carries: the login provider's names, NOT rela's acl.yaml roles, and
	// not the principal's effective permissions.
	Roles []string `json:"roles,omitempty"`
	// Person is the entity the principal resolves to, when the principal may
	// read it. A hidden or missing entity leaves it absent.
	Person *MePerson `json:"person,omitempty"`
	// Links are the configured account links that apply to this principal.
	// Absent when none do.
	Links *MeLinks `json:"links,omitempty"`
	// CanConfigure is true when the server offers the Configure space and
	// the principal may use it (the config:edit permission).
	CanConfigure bool `json:"can_configure,omitempty"`
}

// MeOrg is the principal's org. ID is always set; Slug and Name are omitted
// when their claims are absent.
type MeOrg struct {
	ID   string `json:"id"`
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

// MePerson is the principal's own entity, read through the same row gate
// and field redaction as any other read.
type MePerson struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title"`
	// Avatar is an image URL for an <img src>: a root-relative path (the
	// attachment endpoint, for a `file` avatar property; resolve it against
	// the project base like any other API path) or an https URL. Absent when
	// `account.avatar_property` is not configured, is hidden from this
	// principal, or holds no usable value.
	Avatar string `json:"avatar,omitempty"`
}

// MeLinks are the account-menu links from the `account:` config. Each is a
// root-relative path or an https URL to a page the login proxy owns.
type MeLinks struct {
	SignOut   string `json:"sign_out,omitempty"`
	Account   string `json:"account,omitempty"`
	SwitchOrg string `json:"switch_org,omitempty"`
	// Admin is present when configured and the principal's asserted roles
	// contain `account.admin.role` (or no role is configured). A display
	// filter only: the proxy's admin page authorizes on its own.
	Admin string `json:"admin,omitempty"`
}
