---
id: RR-GSYM8Y
type: review-response
title: IsSameEntity permits from == to, so a guarded same-face copy became an unauthorized in-place write
finding: 'The first cut of the fix keyed the exemption on `plan.def.IsSameEntity() && plan.def.Guard.Permission != ""`. IsSameEntity compares TYPES, not faces (`!to.IsNew && from.Type == to.Type`), so a definition like `from: ticket` / `to: ticket` satisfies it while moving nothing between faces -- on a type that may declare no faces at all. Reproduced: under acl.ReadOnlyACL with a permissive guard, such a copy succeeded and mutated the entity. The old condition correctly refused it, so this was a regression the fix introduced, not a pre-existing hole. The doc comment''s whole argument (nobody holds update on a guarded face; the operator wrote both endpoints) is about moving content between two distinct faces, and neither premise holds when the endpoints coincide.'
severity: critical
resolution: 'Narrowed the condition to `guarded && plan.def.IsSameEntity() && plan.sourceTail != plan.targetTail`, restoring the premise the doc comment argues from. The atlas promote still works (published -> bare draft crosses a boundary) and TestCopy_GuardedFaceStaysExemptFromUpdate still passes. Added TestCopy_GuardDoesNotOverruleASameFaceCopy for the missing matrix cell, and rewrote the doc comment to state all three conditions with the reason each is load-bearing. Mutation-tested: dropping any one of the three clauses now fails a distinct test.'
status: addressed
---
