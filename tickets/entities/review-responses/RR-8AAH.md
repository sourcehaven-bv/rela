---
id: RR-8AAH
type: review-response
title: Request.Visible option (c) re-implements readQuery — divergence is a security bug
finding: 'Plan picks option (c): re-walk member-of × ancestors in Visible directly, rejecting option (a) (`WhereIDs` in GraphQuery) as YAGNI. But Visible must compute exactly the same boolean ReadQuery would for that id under the same predicate composition (conferring relations, InheritThrough endpoints, EntityInheritThrough ancestors, depthCap). They have no structural guarantee to stay in sync — a future extension to readQuery (e.g. wildcard EntityType handling, attribute-based predicates) that doesn''t also touch Visible silently desyncs LIST vs GET. The plan''s mitigation (table-driven test comparing Visible vs ReadQuery+membership) catches today''s regression but not tomorrow''s. Fix: define Visible IN TERMS OF readQuery. Either (a) add `WhereIDs []string` to store.RelationPredicate or GraphQuery (one field, one line in two backends, one storetest case) and have Visible call GraphCount with WhereIDs=[id] and return matched > 0; or (b) extract conferring/predicate composition into a private helper that both readQuery and Visible call. The YAGNI framing here mis-weights cost (small one-time field add) against benefit (single source of truth for read-filter semantics). Reject YAGNI, pick (a).'
severity: significant
resolution: 'Accepted: option (a) - add WhereIDs []string to store.GraphQuery. Visible calls GraphCount(WhereIDs=[id]) and returns matched > 0. readQuery and Visible share predicate composition. Single source of truth. Documented in TKT-VQGN scope bullet 1 + AC for Visible feature test.'
status: addressed
---
