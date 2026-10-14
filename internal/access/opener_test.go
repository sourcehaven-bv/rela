package access

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// lookupFunc is a PrincipalLookup with a fixed answer.
type lookupFunc func(value string) ([]string, error)

func (f lookupFunc) LookupEntityByProperty(_ context.Context, _, _, value string) ([]string, error) {
	return f(value)
}

var quietLog = slog.New(slog.DiscardHandler)

// lookupPolicy returns a policy whose principal_property lookup answers with
// lookup.
func lookupPolicy(t *testing.T, lookup lookupFunc) *acl.Declarative {
	t.Helper()
	p := &acl.Policy{UserEntityType: "person", PrincipalProperty: "sub"}
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{}, acl.WithPrincipalLookup(lookup))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

func mustOpener(t *testing.T, d *acl.Declarative, verifiedIdentity bool) *Opener {
	t.Helper()
	o, err := NewOpener(d, verifiedIdentity)
	if err != nil {
		t.Fatalf("NewOpener: %v", err)
	}
	return o
}

func asUser(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

func TestNewOpener_RejectsNilPolicy(t *testing.T) {
	if _, err := NewOpener(nil, false); err == nil {
		t.Fatal("NewOpener(nil) succeeded, want an error")
	}
}

func TestOpen_AttachesOneRequest(t *testing.T) {
	o := mustOpener(t, mustDeclarative(t), false)
	ctx, req, err := o.Open(asUser("alice"), quietLog)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if req == nil || acl.FromContext(ctx) != req {
		t.Fatal("Open did not attach the request it returned")
	}
	if got := req.Principal().User; got != "alice" {
		t.Errorf("request principal = %q, want alice", got)
	}
}

func TestOpen_ReusesAttachedRequest(t *testing.T) {
	d := mustDeclarative(t)
	o := mustOpener(t, d, false)
	ctx := asUser("alice")
	upstream, err := d.ForPrincipal(principal.From(ctx))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	ctx = acl.WithRequest(ctx, upstream)

	_, req, err := o.Open(ctx, quietLog)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if req != upstream {
		t.Error("Open opened a second request instead of reusing the attached one")
	}
}

func TestOpen_RejectsRequestForAnotherPrincipal(t *testing.T) {
	d := mustDeclarative(t)
	o := mustOpener(t, d, false)
	bob, err := d.ForPrincipal(principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	ctx := acl.WithRequest(asUser("alice"), bob)

	if _, req, err := o.Open(ctx, quietLog); !errors.Is(err, ErrPrincipalMismatch) || req != nil {
		t.Fatalf("Open = (%v, %v), want (nil, ErrPrincipalMismatch)", req, err)
	}
}

func TestOpen_RejectsUnstampedPrincipal(t *testing.T) {
	o := mustOpener(t, mustDeclarative(t), false)
	_, req, err := o.Open(context.Background(), quietLog)
	if !errors.Is(err, ErrUnresolvedPrincipal) || req != nil {
		t.Fatalf("Open = (%v, %v), want (nil, ErrUnresolvedPrincipal)", req, err)
	}
	if !errors.Is(err, acl.ErrUnstampedPrincipal) {
		t.Errorf("Open error %v does not wrap acl.ErrUnstampedPrincipal", err)
	}
}

func TestOpen_ResolvesPrincipalProperty(t *testing.T) {
	tests := []struct {
		name             string
		lookup           lookupFunc
		verifiedIdentity bool
		wantUser         string
		wantRaw          string
		wantUnmatched    bool
	}{
		{
			name:     "one match re-stamps",
			lookup:   func(string) ([]string, error) { return []string{"PERS-1"}, nil },
			wantUser: "PERS-1", wantRaw: "sub-alice",
		},
		{
			name:     "no match keeps the raw principal",
			lookup:   func(string) ([]string, error) { return nil, nil },
			wantUser: "sub-alice",
		},
		{
			name:             "no match on a verified identity is marked unmatched",
			lookup:           func(string) ([]string, error) { return nil, nil },
			verifiedIdentity: true,
			wantUser:         "sub-alice", wantUnmatched: true,
		},
		{
			name:             "a lookup error keeps the raw principal unmarked",
			lookup:           func(string) ([]string, error) { return nil, errors.New("backend down") },
			verifiedIdentity: true,
			wantUser:         "sub-alice",
		},
		{
			name:     "an ambiguous match keeps the raw principal",
			lookup:   func(string) ([]string, error) { return []string{"PERS-1", "PERS-2"}, nil },
			wantUser: "sub-alice",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := mustOpener(t, lookupPolicy(t, tc.lookup), tc.verifiedIdentity)
			ctx, req, err := o.Open(asUser("sub-alice"), quietLog)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			p := principal.From(ctx)
			if p.User != tc.wantUser || p.RawUser != tc.wantRaw {
				t.Errorf("principal = (%q, raw %q), want (%q, raw %q)", p.User, p.RawUser, tc.wantUser, tc.wantRaw)
			}
			if req.Principal().User != tc.wantUser {
				t.Errorf("request principal = %q, want %q", req.Principal().User, tc.wantUser)
			}
			if got := acl.UnmatchedVerifiedFrom(ctx); got != tc.wantUnmatched {
				t.Errorf("unmatched-verified = %v, want %v", got, tc.wantUnmatched)
			}
		})
	}
}

// TestOpen_ResolutionKeepsClaims: the re-stamped principal keeps the asserted
// claims, the client ceiling's scopes included, so resolution never widens a
// restricted client.
func TestOpen_ResolutionKeepsClaims(t *testing.T) {
	o := mustOpener(t, lookupPolicy(t, func(string) ([]string, error) { return []string{"PERS-1"}, nil }), true)
	in := principal.VerifiedFrom("sub-alice", principal.ToolDataEntry, principal.Claims{
		OrgID: "org-1", Roles: []string{"editor"}, Scopes: []string{"rela.read"}, Email: "a@example.org",
	})
	ctx, _, err := o.Open(principal.With(context.Background(), in), quietLog)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	out := principal.From(ctx)
	if out.User != "PERS-1" || out.OrgID() != "org-1" || out.Email() != "a@example.org" ||
		len(out.Roles()) != 1 || len(out.Scopes()) != 1 {

		t.Errorf("resolved principal lost claims: %+v", out)
	}
}

func TestRestamp(t *testing.T) {
	tests := []struct {
		name     string
		lookup   lookupFunc
		wantErr  bool
		wantUser string
	}{
		{name: "one match", lookup: func(string) ([]string, error) { return []string{"PERS-9"}, nil }, wantUser: "PERS-9"},
		{name: "no match", lookup: func(string) ([]string, error) { return nil, nil }, wantErr: true},
		{name: "ambiguous", lookup: func(string) ([]string, error) { return []string{"A", "B"}, nil }, wantErr: true},
		{name: "lookup error", lookup: func(string) ([]string, error) { return nil, errors.New("down") }, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := acl.WithUnmatchedVerified(asUser("sub-new"))
			ctx, req, err := Restamp(in, lookupPolicy(t, tc.lookup), "sub-new")
			if tc.wantErr {
				if !errors.Is(err, ErrUnresolvedPrincipal) || req != nil || ctx != in {
					t.Fatalf("Restamp = (changed ctx %v, %v, %v), want the input ctx, nil, ErrUnresolvedPrincipal",
						ctx != in, req, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Restamp: %v", err)
			}
			p := principal.From(ctx)
			if p.User != tc.wantUser || p.RawUser != "sub-new" {
				t.Errorf("principal = (%q, raw %q), want (%q, raw sub-new)", p.User, p.RawUser, tc.wantUser)
			}
			if acl.UnmatchedVerifiedFrom(ctx) {
				t.Error("restamped ctx is still marked unmatched")
			}
			if acl.FromContext(ctx) != req || req.Principal().User != tc.wantUser {
				t.Error("restamped ctx does not carry the request for the resolved principal")
			}
		})
	}
}

func TestRestamp_RejectsNilPolicy(t *testing.T) {
	in := asUser("sub-new")
	ctx, req, err := Restamp(in, nil, "sub-new")
	if err == nil || req != nil || ctx != in {
		t.Fatalf("Restamp(nil policy) = (%v, %v, %v), want the input ctx, nil, an error", ctx, req, err)
	}
}

func TestOpen_NilLogFallsBackToDefault(t *testing.T) {
	o := mustOpener(t, lookupPolicy(t, func(string) ([]string, error) { return nil, nil }), false)
	if _, req, err := o.Open(asUser("alice"), nil); err != nil || req == nil {
		t.Fatalf("Open(nil log) = (%v, %v), want a request", req, err)
	}
}
