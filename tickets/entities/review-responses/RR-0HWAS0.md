---
id: RR-0HWAS0
type: review-response
title: Property/relation name collision silently mis-routes property filter to relation pass
finding: 'Properties (per-entity-type EntityDef.Properties) and relations (global Metamodel.Relations) are disjoint namespaces; the only guard is ReservedPropertyNames={id,type}. Nothing forbids property taak.belongs_to coexisting with relation belongs_to. When they collide: a list on taak with filter_control {property: belongs_to} sends filter[belongs_to]=X; applyV1Filters skips it (isRelationKey true) and applyRelationFilters treats it as a relation (GetRelationDef ok) — the property filter silently produces wrong rows or an empty list, no warning. Root cause: both passes re-derive relation-ness from the bare query-param name via GetRelationDef instead of using the config discriminator FilterControl.IsRelation() which already distinguishes fc.Property vs fc.Relation. Fix (preferred): thread the config discriminator through the pipeline instead of re-guessing from the param name; OR flag the property/relation name ambiguity at load in CollectConfigWarnings. Reachable, untested. api_v1.go:311,:1934; metamodel/types.go:276.'
severity: significant
resolution: 'relationFilterClassifier threads the config discriminator: an explicit property FilterControl (HasPropertyFilterControl) forces the property pass; a name that is both a property of the type and a relation control resolves to the property (safe). CollectConfigWarnings flags the ambiguity at load (TestCollectConfigWarnings_RelationPropertyNameCollision). Runtime pinned by TestV1ListPropertyRelationNameCollision. Commit 72f10b99.'
status: addressed
---
