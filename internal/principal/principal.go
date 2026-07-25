// Package principal carries the identity attribution attached to each
// request: who initiated it (User) and through what entry point
// (Tool). Entry-point binaries stamp a [Principal] on the request
// context once at startup (or per-request for HTTP-style entry
// points); downstream consumers — audit logging today, access control
// in subsequent PRs — read it via [From].
//
// This package is deliberately small. It owns the [Principal] type,
// the Tool* constants, the context plumbing, and the OS-user lookup.
// It does NOT carry session lifecycle, role mapping, or
// authentication — those are separate concerns that may grow their
// own packages.
//
// **Growth gated on ACL.** The package was extracted from `audit`
// specifically so a future ACL ticket can import [Principal] without
// pulling audit. If that ticket gets de-prioritized and no second
// consumer materializes, this package should be reabsorbed into the
// consumer that uses it most — currently `audit`. Don't grow this
// package speculatively.
//
// That gate has since opened, once (TKT-RP3X3Q): [Principal] gained the
// verified-assertion claims — orgID, orgSlug, roles — because internal/acl
// needs to grant roles from a signed identity assertion, and the claims have
// nowhere else to live that both the ACL evaluator and the audit log can read.
// The unexported-fields-plus-[Verified] shape exists so the trust boundary is
// enforced by the compiler; see the [Principal] doc.
//
// The gate is not now open generally. `roles` earned its place by having an
// evaluator; `orgID`/`orgSlug` ride along for audit attribution only and are
// the outer limit of what belongs here. Session lifecycle, provisioning, and
// role *expansion* remain out — they are separate concerns with their own
// packages.
package principal

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

// Principal identifies who is making a write. User is the OS user
// captured at process startup via [SystemUser]; data-entry overrides it
// per-request from an HTTP middleware. Tool identifies the entry point —
// one of the Tool* constants below.
//
// RawUser records the pre-resolution identifier when the ACL policy's
// `principal_property` lookup substituted User with a graph entity ID
// (e.g. the `X-Forwarded-User` email before it became a `PERS-…` ID). It
// is empty when no substitution happened — the common case — and exists
// so the audit log can record BOTH the identity that authenticated and
// the entity it resolved to without a round-trip to the graph. Old audit
// consumers ignore the `raw_user` key (omitempty).
//
// # Verified assertion claims
//
// orgID, orgSlug, and roles carry claims from a CRYPTOGRAPHICALLY VERIFIED
// identity assertion. They are unexported on purpose: [Verified] is the only
// way to populate them, so no composite literal anywhere in the tree can forge
// a role. That matters because internal/acl trusts a Principal absolutely — it
// verifies nothing itself — so a role reaching it from an unverified source
// (a spoofable header, say) would be a complete authorization bypass. The
// compiler enforces the trust boundary here rather than leaving it to a code
// reviewer's memory.
//
// Read them via [Principal.OrgID], [Principal.OrgSlug], [Principal.Roles].
type Principal struct {
	User    string
	Tool    string
	RawUser string

	orgID   string
	orgSlug string
	roles   []string
}

// Verified constructs a Principal carrying claims from a verified identity
// assertion. Callers MUST have validated the assertion's signature before
// calling this — see the type doc for why that is load-bearing.
//
// roles is defensively copied so a later mutation of the caller's slice cannot
// retroactively change an authorization decision.
func Verified(user, tool, orgID, orgSlug string, roles []string) Principal {
	p := Principal{
		User:    user,
		Tool:    tool,
		orgID:   orgID,
		orgSlug: orgSlug,
	}
	if len(roles) > 0 {
		p.roles = slices.Clone(roles)
	}
	return p
}

// OrgID returns the verified `org_id` claim, or "" when the principal did not
// arrive with a verified assertion.
//
// ATTRIBUTION ONLY. Nothing in internal/acl evaluates this — a principal in
// org A holding a role sees every entity that role grants, in EVERY org.
// Presence of an org in the audit log does NOT imply tenant isolation.
// Enforcement is deliberately deferred; see TKT-RP3X3Q.
func (p Principal) OrgID() string { return p.orgID }

// OrgSlug returns the verified `org_slug` claim, or "" when absent. The same
// attribution-only caveat as [Principal.OrgID] applies.
func (p Principal) OrgSlug() string { return p.orgSlug }

// Roles returns the verified `roles` claim — bare role names scoped to
// [Principal.OrgID]. Empty for every non-assertion entry point (CLI, MCP,
// scheduler, a proxy-trusted header, ...).
//
// These are the IdP's role names, NOT rela roles: an ACL policy maps them to
// declared roles through an operator-authored allowlist, so a claim value can
// never name a rela role the operator did not choose to grant.
//
// The returned slice is a copy; mutating it cannot affect authorization.
func (p Principal) Roles() []string {
	if len(p.roles) == 0 {
		return nil
	}
	return slices.Clone(p.roles)
}

// IsZero reports whether p carries no identity at all. Entry points use it to
// reject an unconfigured Principal at construction time.
//
// This exists because Principal stopped being comparable when it gained a slice
// field, so `p == Principal{}` no longer compiles. Deliberately NOT
// reflect.DeepEqual: a Principal carrying only assertion claims and no
// User/Tool is still an unusable identity and must still be rejected.
func (p Principal) IsZero() bool {
	return p.User == "" && p.Tool == "" && p.RawUser == ""
}

// Clone returns a deep copy of p, safe to hand to code that might outlive or
// mutate the original.
//
// Principal is a value type, so a plain assignment already copies the scalar
// fields — but the roles slice header is shared, and a caller resliced onto
// that backing array could mutate a role another holder is about to authorize
// against. This is the one place the compiler stops enforcing the guarantee the
// unexported fields exist to provide, so accessors that hand out a Principal by
// value use this instead.
func (p Principal) Clone() Principal {
	if len(p.roles) > 0 {
		p.roles = slices.Clone(p.roles)
	}
	return p
}

// Sanitized returns a copy of p with every string field passed through clean.
// Used at a serialization boundary (the audit JSONL writer) to bound field
// length and strip control characters.
//
// It lives here rather than in the consumer because the assertion claims are
// unexported: a consumer cannot reach them, and a consumer that only cleaned
// the exported fields would silently let unbounded role strings through. clean
// is supplied by the caller so this package stays free of a policy decision
// about what "clean" means.
func (p Principal) Sanitized(clean func(string) string) Principal {
	out := Principal{
		User:    clean(p.User),
		Tool:    clean(p.Tool),
		RawUser: clean(p.RawUser),
		orgID:   clean(p.orgID),
		orgSlug: clean(p.orgSlug),
	}
	if len(p.roles) > 0 {
		out.roles = make([]string, 0, len(p.roles))
		for _, r := range p.roles {
			out.roles = append(out.roles, clean(r))
		}
	}
	return out
}

// Equal reports whether p and q carry the same identity, including the
// assertion claims. Replaces `==`, which stopped compiling when Principal
// gained a slice field.
func (p Principal) Equal(q Principal) bool {
	return p.User == q.User &&
		p.Tool == q.Tool &&
		p.RawUser == q.RawUser &&
		p.orgID == q.orgID &&
		p.orgSlug == q.orgSlug &&
		slices.Equal(p.roles, q.roles)
}

// principalJSON is the wire format. It is a separate type because the assertion
// claims live in unexported fields (see the [Principal] doc), which
// encoding/json cannot reach in either direction.
type principalJSON struct {
	User    string   `json:"user"`
	Tool    string   `json:"tool"`
	RawUser string   `json:"raw_user,omitempty"`
	OrgID   string   `json:"org_id,omitempty"`
	OrgSlug string   `json:"org_slug,omitempty"`
	Roles   []string `json:"roles,omitempty"`
}

// MarshalJSON emits the assertion claims alongside the exported fields. Every
// new key is omitempty, so a record from a non-assertion entry point is
// byte-identical to what previous versions wrote.
func (p Principal) MarshalJSON() ([]byte, error) {
	return json.Marshal(principalJSON{
		User:    p.User,
		Tool:    p.Tool,
		RawUser: p.RawUser,
		OrgID:   p.orgID,
		OrgSlug: p.orgSlug,
		Roles:   p.roles,
	})
}

// UnmarshalJSON restores a Principal from the audit wire format.
//
// This is the one path that populates the assertion claims without a signature
// check, and it is safe precisely because of what it is for: reading back a
// record this process already wrote and verified. It must never be pointed at
// request-path input — a Principal decoded from an untrusted payload would
// bypass the guarantee the unexported fields exist to provide.
func (p *Principal) UnmarshalJSON(data []byte) error {
	var w principalJSON
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*p = Principal{
		User:    w.User,
		Tool:    w.Tool,
		RawUser: w.RawUser,
		orgID:   w.OrgID,
		orgSlug: w.OrgSlug,
		roles:   w.Roles,
	}
	return nil
}

// Tool constants — the values that may appear in [Principal.Tool].
// Entry-point wiring references these instead of string literals so
// typos surface at compile time.
const (
	ToolCLI       = "cli"
	ToolMCP       = "mcp"
	ToolDataEntry = "data-entry"
	ToolScheduler = "scheduler"
	ToolDesktop   = "desktop"
	// ToolSync attributes writes applied by the sync API (FEAT-NJ9FEN) so the
	// audit log distinguishes a synced write from a direct data-entry edit.
	ToolSync = "sync"
	// ToolWebhookReceiver attributes writes made by an inbound-webhook handler
	// (e.g. an IdP membership event that provisions a person entity). It is a
	// distinct entry point from data-entry: the write originates from a verified
	// server-to-server callback, not a human at the UI.
	ToolWebhookReceiver = "webhook-receiver"
)

// UserScheduler is the default [Principal.User] for scheduled tasks that
// declare no `run_as` — a FIXED identity, deliberately not the OS user.
//
// The scheduler used to default to [SystemUser] ($USER). That made a job's
// read scope depend on which OS account happened to run `rela scheduler`,
// so the assignment an operator needed in acl.yaml differed per host and
// could not be written down in advance. Worse, a systemd unit typically has
// no $USER, so SystemUser() returned "unknown" — which [acl.Declarative]
// rejects as an unstamped principal, failing the task outright rather than
// merely scoping it.
//
// A fixed principal makes the scheduler's identity a documented, grantable
// constant:
//
//	assignments:
//	  system:scheduler: <a-role-with-read>
//
// It grants nothing by itself (DEC-O59WM4) — privileges still come only
// from acl.yaml. `run_as` continues to override it per task.
const UserScheduler = "system:scheduler"

// principalKey is the unexported context.WithValue key so no other
// package can collide with it or read/write the value outside this
// package's API.
type principalKey struct{}

// With returns a derived context carrying p. Entry points stamp
// Principal once at startup (or per-request for data-entry);
// consumers read it via [From] on each operation.
//
// Call sites read as `principal.With(ctx, ...)` and `principal.From(ctx)` —
// the package name carries the noun so the function names don't stutter.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// From returns the Principal carried by ctx, or
// Principal{User:"unknown", Tool:"unknown"} if none was stamped.
// Returning a default rather than panicking keeps downstream consumers
// best-effort even when a new call site forgets to thread principal
// through — the misattribution is visible (in the audit log, for
// instance), not silent.
func From(ctx context.Context) Principal {
	if v, ok := ctx.Value(principalKey{}).(Principal); ok {
		// Cloned: the ctx value is shared by every reader for the life of the
		// request, so handing out the roles backing array would let one
		// consumer mutate what another is about to authorize against.
		return v.Clone()
	}
	return Principal{User: "unknown", Tool: "unknown"}
}

// SystemUser returns the OS user running this process — $USER
// trimmed, or "unknown" if $USER is unset or whitespace-only. Used by
// entry-point wiring to populate [Principal.User].
//
// The original plan had a four-tier fallback chain ($RELA_ACTOR →
// $USER → git config user.email → "system") — dropped during review
// as over-engineered for what is one bit of operator identity.
// Operators who want a different identity set $USER; data-entry will
// override per-request via HTTP middleware in a follow-up.
func SystemUser() string {
	u := strings.TrimSpace(os.Getenv("USER"))
	if u == "" {
		return "unknown"
	}
	return u
}
