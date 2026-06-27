---
id: RR-EKUNMX
type: review-response
title: CTE prefix uniqueness derived from loop index is correct but fragile to refactor
finding: 'buildVisibilityDisjunction derives CTE-name uniqueness from the positional index over sorted scope types. Correct today, but the invariant is emergent from loop structure rather than local; a future refactor reusing the builder or reordering could collide silently. Suggestion: derive the prefix from the (sanitized) type name.'
severity: nit
reason: Positional-index uniqueness is structurally guaranteed by the single sorted loop; deriving prefixes from type names would require sanitizing arbitrary metamodel type strings into SQL identifiers, reintroducing exactly the collision class being avoided (two types sanitizing to the same identifier). The distinct-prefix invariant is now pinned by TestBuildVisibleSearchSQL_Shape, so a refactor that breaks it fails a no-DB unit test rather than corrupting results silently.
status: wont-fix
---
