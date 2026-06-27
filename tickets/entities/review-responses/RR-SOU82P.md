---
id: RR-SOU82P
type: review-response
title: Scope-map construction rule and unknown-type fail-closed semantics unspecified
finding: 'Who enumerates ''all metamodel types'' is unstated, and entities whose type is absent from the metamodel (permissive storage: removed/typo types are storable states) hit ''absent = DenyAll'' silently. Under NopACL that would HIDE unknown-type entities that are visible today, breaking the NopACL regression. The construction rule (which type set drives the map) and deterministic fail-closed lookup for unknown hit types must be pinned.'
severity: significant
resolution: 'Plan rev 2: scope lookup rule pinned — exact type entry → reserved "*" wildcard entry → DenyAll (fail-closed). Construction rule: dataentry iterates meta.Entities calling ReadQuery per type; under ACL no "*" is emitted so off-metamodel types fail closed (documented); NopACL emits {"*": AllowAll} preserving today''s unknown-type visibility. Conformance case 8 pins both wildcard admission and fail-closed denial.'
status: addressed
---
