---
id: RR-RTJE
type: review-response
title: Grant target names not validated against metamodel — typos silently invert intent (S2)
finding: 'Only when: predicates are compiled (which catches unknown field refs). fg.Field, og.Field/og.Option, rg.Relation are stored verbatim with no metamodel check. A typo `field: stauts` allows the bogus name and closed-world DENIES the real `status` — exact opposite of intent, no startup error. A relation typo (depends_on vs depends-on) emits a verdict for a nonexistent type that never gates. Predicates fail loudly; grant targets should too.'
severity: significant
resolution: 'Added internal/affordances/validate.go: validateField/validateOption/validateRelation run in compileRole before predicate compile. Unknown field, unknown/ non-enum option, unknown relation type, and relation-not-originating-from-type all fail New() with path-prefixed errors joined like predicate compile errors. Tests: TestResolver_New_UnknownFieldTarget_Rejected, _UnknownOptionTarget_, _UnknownRelationTarget_.'
status: addressed
---
