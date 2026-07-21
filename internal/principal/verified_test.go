package principal_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Verified-assertion claims on Principal (TKT-RP3X3Q).

func TestVerified_CarriesClaims(t *testing.T) {
	t.Parallel()
	p := principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme",
		[]string{"admin", "billing"})

	if p.User != "usr_1" || p.Tool != principal.ToolDataEntry {
		t.Errorf("identity not carried: %+v", p)
	}
	if p.OrgID() != "org_a" || p.OrgSlug() != "acme" {
		t.Errorf("org = %q/%q, want org_a/acme", p.OrgID(), p.OrgSlug())
	}
	if want := []string{"admin", "billing"}; !slices.Equal(p.Roles(), want) {
		t.Errorf("Roles = %v, want %v", p.Roles(), want)
	}
}

func TestVerified_NoRolesYieldsNil(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		roles []string
	}{
		{"nil", nil},
		{"empty", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := principal.Verified("usr_1", principal.ToolCLI, "", "", tc.roles)
			if got := p.Roles(); got != nil {
				t.Errorf("Roles = %v, want nil", got)
			}
		})
	}
}

func TestPrincipal_ZeroValueHasNoClaims(t *testing.T) {
	t.Parallel()
	// Every non-assertion entry point builds Principal as a composite literal.
	// Those must carry no claims — which the compiler guarantees, since the
	// fields are unexported. This pins the observable half of that.
	p := principal.Principal{User: "alice", Tool: principal.ToolCLI}

	if p.OrgID() != "" || p.OrgSlug() != "" || p.Roles() != nil {
		t.Errorf("composite-literal Principal carried claims: %+v", p)
	}
}

func TestPrincipal_IsZero(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		p    principal.Principal
		want bool
	}{
		{"zero value", principal.Principal{}, true},
		{"user only", principal.Principal{User: "alice"}, false},
		{"tool only", principal.Principal{Tool: principal.ToolCLI}, false},
		{"rawuser only", principal.Principal{RawUser: "a@b.c"}, false},
		{
			// The case that matters: a Principal carrying ONLY assertion claims
			// is not a usable identity and must still count as zero, so an
			// entry point's required-Principal guard still rejects it.
			"claims but no identity",
			principal.Verified("", "", "org_a", "acme", []string{"admin"}),
			true,
		},
		{
			"claims with identity",
			principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme", nil),
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.IsZero(); got != tc.want {
				t.Errorf("IsZero() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrincipal_Equal(t *testing.T) {
	t.Parallel()
	base := principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme",
		[]string{"admin"})

	for _, tc := range []struct {
		name string
		q    principal.Principal
		want bool
	}{
		{"identical", principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme", []string{"admin"}), true},
		{"different user", principal.Verified("usr_2", principal.ToolDataEntry, "org_a", "acme", []string{"admin"}), false},
		{"different org", principal.Verified("usr_1", principal.ToolDataEntry, "org_b", "acme", []string{"admin"}), false},
		{"different roles", principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme", []string{"billing"}), false},
		{"no roles", principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme", nil), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := base.Equal(tc.q); got != tc.want {
				t.Errorf("Equal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrincipal_AccessorsDoNotShareBackingArray(t *testing.T) {
	t.Parallel()
	// Principal is a value type, so a plain copy shares the roles backing
	// array. Every accessor that hands out a Principal or its roles must clone,
	// or one holder can mutate what another is about to authorize against.
	p := principal.Verified("usr_1", principal.ToolDataEntry, "org", "slug",
		[]string{"admin"})

	t.Run("Clone", func(t *testing.T) {
		t.Parallel()
		c := p.Clone()
		c.Roles()[0] = "superuser" // mutating the accessor result is already safe
		if got := p.Roles(); got[0] != "admin" {
			t.Errorf("original mutated: %v", got)
		}
		// The clone's own backing array must be independent.
		if &c.Roles()[0] == &p.Roles()[0] {
			t.Error("Clone shares the roles backing array")
		}
	})

	t.Run("From(ctx)", func(t *testing.T) {
		t.Parallel()
		ctx := principal.With(context.Background(), p)
		a := principal.From(ctx)
		b := principal.From(ctx)
		// Two independent readers of the same ctx must not alias.
		if len(a.Roles()) > 0 && len(b.Roles()) > 0 && &a.Roles()[0] == &b.Roles()[0] {
			t.Error("two From(ctx) results share the roles backing array")
		}
	})
}

func TestPrincipal_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	want := principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme",
		[]string{"admin", "billing"})
	want.RawUser = "alice@example.com"

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got principal.Principal
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("round-trip: got %+v, want %+v (json: %s)", got, want, data)
	}
}

func TestPrincipal_JSONOmitsAbsentClaims(t *testing.T) {
	t.Parallel()
	// The audit log is a published wire format. A record from a non-assertion
	// entry point must be byte-identical to what earlier versions wrote, so old
	// consumers see no change.
	data, err := json.Marshal(principal.Principal{User: "alice", Tool: principal.ToolCLI})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(data), `{"user":"alice","tool":"cli"}`; got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

func TestPrincipal_Sanitized(t *testing.T) {
	t.Parallel()
	// Stand-in for the audit writer's clean(): uppercase marks that the field
	// was visited, so a field the method forgets shows up as un-uppercased.
	visit := strings.ToUpper

	p := principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "acme",
		[]string{"admin", "billing"})
	p.RawUser = "alice@example.com"

	got := p.Sanitized(visit)

	for _, tc := range []struct{ name, got string }{
		{"User", got.User},
		{"Tool", got.Tool},
		{"RawUser", got.RawUser},
		{"OrgID", got.OrgID()},
		{"OrgSlug", got.OrgSlug()},
	} {
		if tc.got != strings.ToUpper(tc.got) {
			t.Errorf("%s was not passed through clean: %q", tc.name, tc.got)
		}
	}
	for i, role := range got.Roles() {
		if role != strings.ToUpper(role) {
			t.Errorf("role[%d] was not passed through clean: %q", i, role)
		}
	}
	if len(got.Roles()) != 2 {
		t.Errorf("Roles = %v, want 2 entries", got.Roles())
	}
}
