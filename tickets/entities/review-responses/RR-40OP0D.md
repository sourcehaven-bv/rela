---
id: RR-40OP0D
type: review-response
title: Get's byte-exact key claim had no test behind it
finding: The sqlitecomments.Get doc comment asserts that both key halves are matched with `=`, which is byte-exact in SQLite unlike LIKE's ASCII case-folding. That is a claim about backend behaviour with nothing pinning it. RunKeyFidelityTests exists precisely because SQLite's LIKE folds case while `=` does not, and it covered Rename and DeleteAllFaces but not Get.
severity: significant
resolution: Added `Get does not resolve a target differing only by case` to RunKeyFidelityTests, reusing its existing seed helper (TKT-1 and tkt-1, both on the draft face). Asserts the correct comment comes back and that each id refuses the other's comment. Placed in RunKeyFidelityTests rather than RunGetTests because filecomments cannot honour it on a case-insensitive filesystem — which is why that suite is split out. Passes on both database backends.
status: addressed
---

## Finding

The `sqlitecomments.Get` doc comment states:

> Both halves of the key are matched with `=`, which is byte-exact in SQLite —
> unlike LIKE, whose ASCII case-folding the face-prefix queries in this file
> have to defend against.

True, and untested. `RunKeyFidelityTests` exists for exactly this hazard — it
was written because SQLite's `LIKE` is ASCII case-insensitive by default while
`=` is byte-exact, so the two arms of one query silently match different row
sets — and it covered `Rename` and `DeleteAllFaces` but not `Get`.

A property asserted in prose and not in a test is one refactor away from being
false, and this one governs whether a lookup can reach an entity the caller did
not name.

## Resolution

Added `Get does not resolve a target differing only by case` to
`RunKeyFidelityTests`, reusing the existing `seed` helper (which already stores
`TKT-1` and `tkt-1`, both on the draft face). It asserts the right comment comes
back for the right id, and that each id refuses the other's comment.

Placed in `RunKeyFidelityTests` rather than `RunGetTests` deliberately:
`filecomments` keys on a filename, so on a case-insensitive filesystem the two
ids ARE one file. That is a sound decision for a file backend, not a defect, and
it is why the suite is split. Passes on both database backends.
