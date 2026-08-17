---
id: RR-B5Y6DZ
type: review-response
title: Name→property registry needs a deterministic fallback for a miss
finding: The index-name→property registry (rebuilt at store-open) can miss on reload or rolling deploy, and a one-way hash can't be parsed back. Recompute candidate hashes from the CURRENT metamodel and match the incoming ConstraintName instead of a persisted registry; define a safe fallback (generic 409/422, no property) for an unmappable rela_derived_uniq__* — never a 500 or mis-attributed property.
severity: significant
resolution: 'No persisted registry: mapUniqueViolation recomputes uniqueIndexName(T,P) for every current-metamodel unique pair (published via SetUniqueSpecProvider, an atomic.Pointer swapped on reload) and matches the incoming ConstraintName. An owned-prefix index that matches no current pair (rolling deploy / peer-created) degrades to a property-less store.UniquePropertyError{} — still an ErrConflict (via its Is method), never a 500 or misattributed property. Verified by TestMapUniqueViolation ''owned but unknown degrades to property-less''.'
status: addressed
---

Step 3/5 rebuild an index-name→(type,property) registry at store-open to map a
violation's ConstraintName back to a Property. The plan doesn't specify registry
misses: (a) a metamodel reload changes declared uniques but the registry is only
rebuilt "at store-open" — rebuilt on reload? If not, a violation on a
freshly-added rule can't map. (b) rolling deploy: process B (new metamodel)
created rela_derived_uniq__Y; process A (old registry lacks Y) gets 23505 on Y
and can't map. (c) a one-way hash forbids parsing the name back.

REQUIRED: recompute the mapping deterministically from the CURRENT metamodel
(compute all candidate hashes from declared uniques and match the incoming
ConstraintName) instead of relying on a persisted registry — plus define the
fallback: an unmappable rela_derived_uniq__* violation degrades to a safe
generic error (409/422, no property name), NEVER a 500 or mis-attributed
property.
