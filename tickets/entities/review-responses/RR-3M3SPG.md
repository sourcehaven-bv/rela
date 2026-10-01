---
id: RR-3M3SPG
type: review-response
title: 'Closed-world visible: logic would be duplicated in acl'
finding: Field visibility (cross-role opt-in, universe, ceiling intersection) lives in internal/affordances; a copy in acl would drift from runtime.
severity: significant
resolution: 'Plan adds a static entry point in affordances (StaticFieldVisibility / StaticRelationFieldVisibility) reusing dimension, with when: treated as passing, plus an agreement test against FieldVerdicts.'
status: addressed
---
