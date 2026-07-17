---
id: RR-EFFDQL
type: review-response
title: principal_property resolution path is entirely untested
finding: 'No test in aclmap or cli configures user_entity_type + principal_property. The raw->resolved substitution, Raw-field population, ambiguous-match error propagation, and the duplicate-row scenario have zero coverage — and this is the path the CLI doc flags as the scope caveat, i.e. the subtle one most likely to be wrong. A no-false-negative tool must test a resolvable principal end-to-end. Fix: add a resolution test asserting exactly one correctly-attributed row with Raw populated.'
severity: significant
resolution: 'Added TestWhoCan_PrincipalPropertyResolution: configures user_entity_type+principal_property (unique email), validates the policy against a metamodel (the boot gate), wires NewStorePrincipalLookup, and asserts a resolvable principal produces exactly one correctly-attributed row via its role-relation route. Added buildWorldWithMeta + metaView test helpers for the resolution path.'
status: addressed
---
