---
id: RR-6S4YWO
type: review-response
title: related() subject is not restricted to the row entity
finding: walkRelated accepts any record-typed subject, including current_user, and TraversalSpec does not record the subject. Validation and index derivation both resolve from the scope's type, so related(current_user, ...) validates against the wrong type.
severity: significant
resolution: 'TraversalSpec records Subject. ValidateTraversals and ResolveTraversal refuse any subject other than entity at load, and traversalFunc checks the subject record''s id equals the row being evaluated. Tests: TestValidateTraversals_RefusesAnotherSubject, TestTraversalFunc_Refuses.'
status: addressed
---
