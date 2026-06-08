---
id: RR-Z4AP
type: review-response
title: Visible/GraphQueryer placement on Declarative is unpinned — drives test ergonomics
finding: 'Plan says ''Request needs a GraphQueryer dependency — pass via Request construction.'' Three options: (1) Declarative.ForPrincipal(p, gq) explicit per-call; (2) Declarative gains a graphQueryer field at NewDeclarative time; (3) Request has a graphQueryer field set by middleware. Option 2 is right — matches the existing `graph Graph` field on Declarative (same lifetime, same composition root, no per-call churn). Option 1 forces every test to plumb a GraphQueryer through ForPrincipal even when only exercising Globals/ForEntity. Option 3 splits Request invariants across two construction sites. Pin in plan: GraphQueryer is a NewDeclarative constructor parameter, validated non-nil, parallel to Graph. Also document in Visible''s godoc that readQuery returns a *GraphQuery for the caller to execute while Visible executes internally — asymmetry is intentional, don''t refactor.'
severity: significant
resolution: 'NewDeclarative(policy, graph, graphQueryer) takes GraphQueryer as a 3rd constructor parameter validated non-nil; Declarative.graphQueryer is the field used by Request.PermitsRead / PermitsReadMany. Architect rework went further: dropped the asymmetric *GraphQuery return + caller-executes pattern; the gate now executes via GraphQueryer.MatchingIDs so there''s no asymmetry to document.'
status: addressed
---
