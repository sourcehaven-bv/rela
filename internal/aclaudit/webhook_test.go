package aclaudit_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclaudit"
)

func TestCheckWebhooks(t *testing.T) {
	t.Parallel()
	const roles = `
roles:
  hook: { read: [incident], create: [incident] }
`
	for _, tc := range []struct {
		name   string
		policy string
		hooks  []string
		want   bool
	}{
		{"hooks without a webhook baseline", roles, []string{"b-hook", "a-hook"}, true},
		{"baseline for another type does not count", roles + `
client_baselines:
  apps: { applies_to: [app], deny_write: ["*"] }
`, []string{"a-hook"}, true},
		{"webhook baseline caps the hooks", roles + `
client_baselines:
  hooks: { applies_to: [webhook], read: [incident] }
`, []string{"a-hook"}, false},
		{"no hooks, no finding", roles, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := acl.LoadPolicyBytes([]byte(tc.policy))
			if err != nil {
				t.Fatalf("load policy: %v", err)
			}
			got := aclaudit.CheckWebhooks(p, tc.hooks)
			if (len(got) == 1) != tc.want || len(got) > 1 {
				t.Fatalf("findings = %+v, want finding: %v", got, tc.want)
			}
			if !tc.want {
				return
			}
			f := got[0]
			if f.Rule != "A15-webhook-uncapped" || f.Severity != aclaudit.Medium {
				t.Errorf("finding = %s/%v, want A15-webhook-uncapped/medium", f.Rule, f.Severity)
			}
			if !strings.Contains(f.Detail, strings.Join(slices.Sorted(slices.Values(tc.hooks)), ", ")) {
				t.Errorf("detail %q does not list the hooks sorted", f.Detail)
			}
		})
	}
}

func TestCheckWebhooks_NilPolicy(t *testing.T) {
	t.Parallel()
	if got := aclaudit.CheckWebhooks(nil, []string{"a"}); got != nil {
		t.Errorf("CheckWebhooks(nil) = %+v, want nil", got)
	}
}
