---
id: RR-2IKYBT
type: review-response
title: 'resolvingSteps entry would enforce nothing and would break the happy path'
finding: 'validateDeltasResolved (file.go:180-189) populates its present map by type-switching on *migrateFaceStep alone, keyed by cf.Entity, a bare entity-type name. Relation deltas carry a prefixed subject, rel:<name> (shapecompare.go:250). A reverseRelationStep therefore contributes nothing to the map, so a non-empty resolvingSteps entry refuses EVERY file spanning the delta - including a correct one carrying the step. The generated draft would not parse. The existing guard test TestResolvingSteps_CoversEveryMigrationDeltaKind does not catch this: it asserts only that the delta kind is LISTED, never that the enforcement fires.'
severity: critical
resolution: 'Found independently and fixed in the plan before the review returned. The population loop is generalized via a subject-reporting seam that both step kinds implement, returning cf.Entity and rel:+s.Type respectively, and the type switch is replaced by that interface. Acceptance criterion 7 strengthened to assert the refusal actually fires for the relation delta and that a correct file parses, rather than asserting the map has an entry.'
status: addressed
---
