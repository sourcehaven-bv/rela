---
id: RR-0DLHGU
type: review-response
title: ValidateProperty skip dropped live clone-path inputs
finding: 'The first version of the fix copied FuzzPropertyValuesTypeZoo''s `if storeutil.ValidateProperty(propName) != nil { return }` guard into FuzzCloneNestedValues. That guard''s premise does not transfer. TypeZoo skips empty and slash-containing property names because it calls s.PropertyValues, which is the surface where the BUG-CQYD5X backend divergence lives (only sqlitestore enforces the rule, and only there). FuzzCloneNestedValues never calls PropertyValues — it only does CreateEntity, GetEntity, mutate, GetEntity — so that divergence cannot manifest on this path. The skip therefore bought nothing and cost real coverage: both keys are accepted by the stores and round-trip a nested map, so they were live exercises of the deep-clone path the target exists to test. A slash in a property name is a plausible aliasing hazard precisely because fsstore derives attachment paths from property names. Copying a guard without re-checking its premise is the same class as this bug''s own why5.'
severity: significant
resolution: 'Removed the ValidateProperty skip. Replaced it with the tolerate-stricter shape already used by createEntityOrSkip and FuzzAttachmentKeyCollision: assert on the shared rule (ValidateProperties), then tolerate a stricter backend''s refusal with a bare `if err != nil { return }` — so an accepted name still reaches the clone assertions. Verified with a probe that both "" and "a/b" are accepted by memstore and now flow into the clone assertions rather than being skipped. Comment records why the skip would be wrong here.'
status: addressed
---

## Finding

Raised by cranky-code-reviewer against the first version of the branch.

## Verification

Confirmed both claims directly before acting:

- `FuzzCloneNestedValues` contains no `PropertyValues` call; `FuzzPropertyValuesTypeZoo` does.
- A probe against memstore showed `""` and `"a/b"` are both accepted and
round-trip a nested map, so the skip was discarding inputs that exercise the
clone path.

## Fix

Uses the tolerate-stricter pattern from `FuzzAttachmentKeyCollision`
(`internal/store/storetest/fuzz.go`) rather than an outright skip.
