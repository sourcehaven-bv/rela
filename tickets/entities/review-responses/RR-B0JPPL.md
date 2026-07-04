---
id: RR-B0JPPL
type: review-response
title: Any metamodel relation is filterable, not just configured filter_controls
finding: 'applyRelationFilters gates only on meta.GetRelationDef(relation) existing; it never checks a FilterControl for that relation is configured on the list, and RelationFilterDirection''s ok return is discarded (direction, _ :=). Verified: with no controls configured at all, `filter[belongs_to]=Apollo` still filters. A caller can filter any list by any schema relation whether or not the UI exposes it. Combined with the ACL finding this is the leak vector. Decide whether this permissive behavior is intended (like property filters) or should be restricted to configured controls; document + test either way. api_v1.go:311,:318.'
severity: significant
resolution: 'Relation filtering is now restricted to configured filter_controls via relationFilterClassifier, which uses RelationFilterDirection''s ok return: a filter[<rel>] param routes to the relation pass only when a control on a list of this type configures it; otherwise it falls through to the property pass (fail-closed). Pinned by TestV1ListUncontrolledRelationNotFilterable. Commit 72f10b99.'
status: addressed
---
