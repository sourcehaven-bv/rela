package principal_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Webhook principals carry principal_type webhook (TKT-J3RDJ4).

func TestWebhook_IdentityAndType(t *testing.T) {
	t.Parallel()
	p := principal.Webhook("icinga-alert")
	if p.User != "system:webhook:icinga-alert" {
		t.Errorf("User = %q, want system:webhook:icinga-alert", p.User)
	}
	if p.Tool != principal.ToolWebhookReceiver {
		t.Errorf("Tool = %q, want %q", p.Tool, principal.ToolWebhookReceiver)
	}
	if got := p.PrincipalType(); got != principal.TypeWebhook {
		t.Errorf("PrincipalType = %q, want %q", got, principal.TypeWebhook)
	}
	if !principal.IsReserved(p.User) {
		t.Error("a webhook identity must stay under the reserved prefix")
	}
}

// TestPrincipalType_DerivedFromWebhookName pins that the name decides the type.
// A scheduler run_as or an `acl can --principal system:webhook:x` builds the
// principal from the bare name; it must meet the same ceiling as a delivery.
func TestPrincipalType_DerivedFromWebhookName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		p    principal.Principal
		want string
	}{
		{"bare webhook name", principal.Principal{User: "system:webhook:x"}, principal.TypeWebhook},
		{"claim cannot override the name",
			principal.VerifiedFrom("system:webhook:x", principal.ToolCLI,
				principal.Claims{PrincipalType: "user"}),
			principal.TypeWebhook},
		{"empty hook id is not a webhook", principal.Principal{User: "system:webhook:"}, ""},
		{"unreserved lookalike is not a webhook", principal.Principal{User: "webhook:x"}, ""},
		{"other system identity keeps its claim",
			principal.VerifiedFrom("system:automation", principal.ToolCLI,
				principal.Claims{PrincipalType: "service"}),
			"service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.PrincipalType(); got != tc.want {
				t.Errorf("PrincipalType() = %q, want %q", got, tc.want)
			}
		})
	}
}
