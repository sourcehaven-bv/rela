package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// requestFor opens a Request for alice under the given policy.
func requestFor(t *testing.T, p *acl.Policy) *acl.Request {
	t.Helper()
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	req, err := d.ForPrincipal(principal.Principal{User: "alice", Tool: "test"})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	return req
}

// TestExistingGrantsUnchangedByFaceField is the CENTERPIECE of this
// change, and the property that makes it safe to land under every existing
// caller.
//
// [acl.EntitySubject] gained a Face, and decideFromAttrs switched from
// grantsVerb to GrantsVerbOnState. Every existing write site constructs an
// EntitySubject WITHOUT naming a face, so it arrives here as the zero
// value — the default face. This asserts that such a write gets byte-for-byte
// the verdict it got before: a bare-type grant still authorizes it, a
// wildcard still authorizes it, and an unrelated grant still does not.
//
// If this ever fails, the change has altered the meaning of grants in every
// deployed acl.yaml — which is the one thing it must not do.
func TestExistingGrantsUnchangedByFaceField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		grants    acl.RoleDef
		op        acl.Op
		target    string
		wantAllow bool
	}{
		{
			name:   "bare-type update grant authorizes an unfaced update",
			grants: acl.RoleDef{Read: []string{"page"}, Update: []string{"page"}},
			op:     acl.OpUpdate, target: "page", wantAllow: true,
		},
		{
			name:   "wildcard update grant authorizes an unfaced update",
			grants: acl.RoleDef{Read: []string{"*"}, Update: []string{"*"}},
			op:     acl.OpUpdate, target: "page", wantAllow: true,
		},
		{
			name:   "a grant on another type does not authorize",
			grants: acl.RoleDef{Read: []string{"ticket"}, Update: []string{"ticket"}},
			op:     acl.OpUpdate, target: "page", wantAllow: false,
		},
		{
			name:   "create grant authorizes create",
			grants: acl.RoleDef{Create: []string{"page"}},
			op:     acl.OpCreate, target: "page", wantAllow: true,
		},
		{
			name:   "delete grant authorizes delete",
			grants: acl.RoleDef{Read: []string{"page"}, Delete: []string{"page"}},
			op:     acl.OpDelete, target: "page", wantAllow: true,
		},
		{
			name:   "rename routes through the update grant",
			grants: acl.RoleDef{Read: []string{"page"}, Update: []string{"page"}},
			op:     acl.OpRename, target: "page", wantAllow: true,
		},
		{
			name:   "no grants at all denies",
			grants: acl.RoleDef{},
			op:     acl.OpUpdate, target: "page", wantAllow: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := requestFor(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"r": tc.grants},
				Assignments: map[string]string{"alice": "r"},
			})
			// EntitySubject WITHOUT a Face — exactly how every existing
			// call site constructs it.
			d := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
				Op:      tc.op,
				Subject: acl.EntitySubject{Type: tc.target, ID: "PAGE-1"},
			})
			if d.Allow != tc.wantAllow {
				t.Errorf("Allow = %v, want %v (reason: %s)", d.Allow, tc.wantAllow, d.Reason)
			}
		})
	}
}

// TestFaceSubjectIsFaceGranular pins the capability the field exists for:
// a state grant authorizes the face it names and nothing else.
//
// This is what makes "a guarded state is writable only via copy definitions"
// expressible at all — before this, `update: ["page@draft"]` authorized
// nothing (grantsVerb skipped it) and `update: ["page"]` authorized every
// face, so there was no way to say "draft yes, published no".
func TestFaceSubjectIsFaceGranular(t *testing.T) {
	t.Parallel()
	draft := entity.Face("draft")
	published := entity.Face("published")

	tests := []struct {
		name      string
		update    []string
		read      []string // defaults to ["page"]; the wildcard cases need ["*"]
		face      entity.Face
		wantAllow bool
	}{
		{name: "state grant authorizes its own face", update: []string{"page@draft"}, face: draft, wantAllow: true},
		{name: "state grant does NOT authorize a sibling face", update: []string{"page@draft"}, face: published, wantAllow: false},
		{name: "state grant does NOT authorize the default face", update: []string{"page@draft"}, face: "", wantAllow: false},
		{name: "bare grant authorizes the default face", update: []string{"page"}, face: "", wantAllow: true},
		{name: "bare grant does NOT authorize a named face", update: []string{"page"}, face: draft, wantAllow: false},
		{
			// The invariant the whole feature turns on: nobody holds update
			// on published by accident. `update: ["*"]` is the grant every
			// admin policy already has, and it must NOT acquire authority
			// over every face the moment a type declares faces.
			name:   "the wildcard does NOT authorize a named face",
			update: []string{"*"}, read: []string{"*"},
			face: published, wantAllow: false,
		},
		{
			name:   "the wildcard still authorizes the default face",
			update: []string{"*"}, read: []string{"*"}, face: "", wantAllow: true,
		},
		{
			name:   "both faces granted explicitly",
			update: []string{"page@draft", "page@published"}, face: published, wantAllow: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			read := tc.read
			if read == nil {
				read = []string{"page"}
			}
			req := requestFor(t, &acl.Policy{
				Roles: map[string]acl.RoleDef{
					"r": {Read: read, Update: tc.update},
				},
				Assignments: map[string]string{"alice": "r"},
			})
			d := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
				Op: acl.OpUpdate,
				Subject: acl.EntitySubject{
					Type: "page", ID: "PAGE-1", Face: tc.face,
				},
			})
			if d.Allow != tc.wantAllow {
				t.Errorf("update %v on face %q: Allow = %v, want %v (reason: %s)",
					tc.update, tc.face, d.Allow, tc.wantAllow, d.Reason)
			}
		})
	}
}

// TestCeilingStillClampsFaceGranularWrites pins that the client ceiling is
// consulted before any role, unchanged by the face.
//
// The ceiling can only ever REMOVE, and it is checked ahead of the role loop
// precisely so a wildcard grant cannot slip past it. Adding a face dimension
// must not create a path around that.
func TestCeilingStillClampsFaceGranularWrites(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"r": {Read: []string{"page"}, Update: []string{"page@draft"}},
		},
		Assignments: map[string]string{"alice": "r"},
		ClientBaselines: map[string]acl.ClientBaseline{
			"app": {
				AppliesTo:   []string{"app"},
				Restriction: acl.Restriction{DenyUpdate: []string{"page"}},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	req, err := d.ForPrincipal(principal.VerifiedFrom("alice", "test",
		principal.Claims{PrincipalType: "app"}))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	dec := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
		Op: acl.OpUpdate,
		Subject: acl.EntitySubject{
			Type: "page", ID: "PAGE-1", Face: entity.Face("draft"),
		},
	})
	if dec.Allow {
		t.Error("a ceiling denying update on page must deny every FACE of page — " +
			"the ceiling is checked before any role and can only remove")
	}
	if dec.RuleKind != "client-ceiling" {
		t.Errorf("the denial must name the ceiling, not a role: RuleKind=%q", dec.RuleKind)
	}
}

// TestCeilingClampsOnTypeNotLiteral pins the fix for the clamp bug this PR
// made reachable.
//
// A ceiling names entity TYPES. Before the fix, filterTypes compared whole
// literals, so a state grant ("page@draft") never matched a ceiling entry
// ("page") and was silently deleted from the role — leaving the face
// permanently unwritable, with the denial naming the ROLE rather than the
// ceiling. An operator would inspect an acl.yaml whose grant was plainly
// present and find nothing wrong with it.
//
// The inversion is the part worth remembering: the wildcard branch returns
// early, so `update: ["*"]` KEPT the state grant while the narrower
// `update: ["page"]` destroyed it.
//
// This was invisible before this PR because grantsVerb skipped state grants
// anyway. It matters now, and it matters most for the copy kernel, whose
// whole premise — "published is writable only via copy definitions" — is
// expressed as a state grant.
func TestCeilingClampsOnTypeNotLiteral(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		restriction acl.Restriction
		wantAllow   bool
		wantRule    string
	}{
		{
			name:        "an allowlist naming the type permits its faces",
			restriction: acl.Restriction{Update: []string{"page"}},
			wantAllow:   true, wantRule: "role-grant",
		},
		{
			name:        "a wildcard allowlist permits its faces",
			restriction: acl.Restriction{Update: []string{"*"}},
			wantAllow:   true, wantRule: "role-grant",
		},
		{
			name:        "a DENIAL on the type reaches every face",
			restriction: acl.Restriction{DenyUpdate: []string{"page"}},
			wantAllow:   false, wantRule: "client-ceiling",
		},
		{
			name:        "an allowlist naming another type denies",
			restriction: acl.Restriction{Update: []string{"ticket"}},
			wantAllow:   false, wantRule: "client-ceiling",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &acl.Policy{
				Roles: map[string]acl.RoleDef{
					"r": {Read: []string{"page"}, Update: []string{"page@draft"}},
				},
				Assignments: map[string]string{"alice": "r"},
				ClientBaselines: map[string]acl.ClientBaseline{
					"app": {AppliesTo: []string{"app"}, Restriction: tc.restriction},
				},
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("policy must load: %v", err)
			}
			d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
			if err != nil {
				t.Fatalf("NewDeclarative: %v", err)
			}
			req, err := d.ForPrincipal(principal.VerifiedFrom("alice", "test",
				principal.Claims{PrincipalType: "app"}))
			if err != nil {
				t.Fatalf("ForPrincipal: %v", err)
			}
			dec := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
				Op: acl.OpUpdate,
				Subject: acl.EntitySubject{
					Type: "page", ID: "PAGE-1", Face: entity.Face("draft"),
				},
			})
			if dec.Allow != tc.wantAllow {
				t.Errorf("Allow = %v, want %v (reason: %s)", dec.Allow, tc.wantAllow, dec.Reason)
			}
			if dec.RuleKind != tc.wantRule {
				t.Errorf("RuleKind = %q, want %q — a denial must name the layer "+
					"that actually caused it, or the operator debugs the wrong file",
					dec.RuleKind, tc.wantRule)
			}
		})
	}
}

// TestCreateWithoutIDStillAuthorized covers the OTHER branch of
// authorizeEntityWrite's `s.ID != ""` fork: a create carries no id yet, so it
// resolves globals-only and never runs the local-role probes.
//
// The compatibility test above passes an ID on every case, which means it
// exercises one of the two branches. This is the one a real create takes.
func TestCreateWithoutIDStillAuthorized(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		create    []string
		wantAllow bool
	}{
		{"bare-type create grant, no id", []string{"page"}, true},
		{"wildcard create grant, no id", []string{"*"}, true},
		{"create grant on another type, no id", []string{"ticket"}, false},
		{"no create grant, no id", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := requestFor(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"r": {Create: tc.create}},
				Assignments: map[string]string{"alice": "r"},
			})
			d := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
				Op:      acl.OpCreate,
				Subject: acl.EntitySubject{Type: "page"}, // no ID, no Face
			})
			if d.Allow != tc.wantAllow {
				t.Errorf("Allow = %v, want %v (reason: %s)", d.Allow, tc.wantAllow, d.Reason)
			}
		})
	}
}
