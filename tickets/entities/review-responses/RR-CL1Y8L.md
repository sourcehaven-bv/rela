---
id: RR-CL1Y8L
type: review-response
title: dataentryconfig behaviour change is half tested and not documented
finding: 'The yamlscalar move changes both dataentryconfig call sites: a | or > block holding yes/no/on/off is now refused. action_scope_test.go:203 adds a case for ActionConfirm only. DashboardRef.UnmarshalYAML (dashboard.go:75) changed in the same way, but no test pins it. GUIDE-data-entry.md does not mention that a block scalar is treated as unquoted. Fix: add a ''block scalar yes'' case to the DashboardRef test table. In GUIDE-data-entry, where the unquoted-boolean rule is described, add that a block scalar counts as unquoted and that only quotes make it text.'
severity: minor
resolution: Added a block scalar yes case (and a !!str yes case) to TestDashboardRef_YAML; GUIDE-data-entry now says, at the dashboard rules and at the action confirm row, that an unquoted yes/no/on/off/y/n is refused, that a block scalar counts as unquoted, and that quotes or !!str make it text.
status: addressed
---
