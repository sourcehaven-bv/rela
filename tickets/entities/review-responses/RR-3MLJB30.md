---
id: RR-3MLJB30
type: review-response
title: RELA_AFFORDANCE_PROFILE can switch the outside_edits ceiling off on REST
finding: 'ResolverFromProfile refuses the none/demo profile only when acl.yaml has field grants (affordances_stub.go:71-99). With outside_edits and no fields: grants the variable would return the Nop resolver and the REST PATCH (validateFieldWrite) would not enforce the ceiling.'
severity: critical
resolution: 'Addressed in plan: the profile override is refused when acl.yaml has grants or a flow declares outside_edits; test added.'
status: addressed
---
