---
id: RR-VOYW
type: review-response
title: Entity.Clone Inaccessible deep-copy semantics not tested
finding: entity.go:140-143 / 200-203 Clone copies Inaccessible via a slice copy. No test covers this. A future 'optimization' replacing `copy(clone.Inaccessible, e.Inaccessible)` with `clone.Inaccessible = e.Inaccessible` would pass every existing test but introduce shared-backing-array aliasing. Add a Clone test that asserts mutating the clone's Inaccessible doesn't affect the original. Same for Relation.Clone.
severity: minor
resolution: Added TestCloneInaccessibleIsolation in entity_test.go that asserts mutating clone.Inaccessible[0].Name does not affect the original. Same for Relation.Clone.
status: addressed
---
