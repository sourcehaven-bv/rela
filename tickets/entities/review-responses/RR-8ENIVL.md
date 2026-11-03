---
id: RR-8ENIVL
type: review-response
title: Policy-wide consumers should mention inert blocks
finding: ConditionallyVisible, FieldRestricted, QueryScopeVisibilityConflicts and the traversal filter refusal stay sound but become more conservative; their comments describe the per-principal closed world and should say a block can now be inert.
severity: nit
resolution: ConditionallyVisible and QueryScopeVisibilityConflicts comments explain why the policy-wide answer stays correct under the union. The affordances package doc states the union rule.
status: addressed
---
