package aclaudit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// CheckWebhooks is A15: the project declares webhooks and no client baseline
// covers principal_type `webhook`, so every hook holds its role's full grants
// plus everything the `everyone` role grants. A hook is an unattended caller
// driven by an external system; the baseline is what bounds it below its
// role.
//
// hookIDs are the keys under `webhooks:` in data-entry.yaml. The caller
// supplies them, so this package stays free of a data-entry import. A
// baseline that covers `webhook` but restricts nothing is A11's finding, not
// this one.
func CheckWebhooks(p *acl.Policy, hookIDs []string) []Finding {
	if p == nil || len(hookIDs) == 0 {
		return nil
	}
	for _, name := range sortedBaselineNames(p) {
		if slices.Contains(p.ClientBaselines[name].AppliesTo, acl.PrincipalTypeWebhook) {
			return nil
		}
	}
	ids := slices.Clone(hookIDs)
	slices.Sort(ids)
	return []Finding{{
		Rule: "A15-webhook-uncapped", Severity: Medium, Subject: "webhooks",
		Detail: fmt.Sprintf("webhook(s) %s run with principal_type %q, and no client baseline "+
			"covers it, so each hook holds its role's full access plus the everyone role's",
			strings.Join(ids, ", "), acl.PrincipalTypeWebhook),
		Fix: fmt.Sprintf("add a client baseline with applies_to: [%s] that allows only what the hooks "+
			"write, and check each hook with `rela acl map --webhook <id> --expect <file>`",
			acl.PrincipalTypeWebhook),
	}}
}
