---
id: BUG-J2E6VJ
type: bug
title: Guarded copy into the bare face is refused unless the caller can already write the target
description: authorizeCopy exempted a same-entity copy from the ACL write check based on which face it targeted rather than on whether it carried a guard, so a guarded promote into the bare face demanded `update` on the target type -- the very grant that makes the face editable by hand.
priority: high
effort: s
why1: authorizeCopy's same-entity exemption tested `plan.targetTail != ""` (target is a non-bare face) instead of `plan.def.Guard.Permission != ""` (the copy carries a guard).
why2: 'The condition and its own doc comment disagreed: the comment justified the exemption by the guard being mandatory at load for non-bare targets, but the code keyed on the target face, which is only a proxy for that.'
why3: The proxy held for every shape exercised at the time. A deployment that makes the bare face the published text was not among them, so the two conditions never diverged in a test.
why4: 'The load-time rule ''a non-bare target must declare a guard'' makes target-face and guard-presence coincide in one direction, which made the proxy look like an equivalence rather than an implication. The first fix then repeated the pattern in the other direction: it replaced the face proxy with `IsSameEntity()`, which compares TYPES not faces, so it silently admitted `from: t` / `to: t` -- a copy crossing no face boundary at all.'
why5: Both the bug and its first fix wrote the authorization predicate in terms of a structural property that was NEAR the security property rather than the security property itself. The rule that actually holds is 'a guarded copy that MOVES content between two declared faces of one entity is authorized by its guard'; every shorter spelling of it (target face, same type) admits or excludes a case the reasoning never covered.
prevention: 'The condition now names all three parts of the rule (guard present, same entity, source face != target face) and the doc comment states why each is load-bearing, including what breaks if it is dropped. Mutation-tested: removing any one clause fails a distinct test (TestCopy_GuardDoesNotOverruleASameFaceCopy, TestCopy_GuardDoesNotOverruleACrossEntityWrite, TestCopy_IntoTheBareFaceNeedsUpdate). The scoping tests assert WHICH rule refused rather than that some refusal happened, so they cannot pass under a blanket deny.'
status: done
---

## Description

`authorizeCopy` exempts a same-entity copy from the ACL write check (check 3)
based on which face it targets, not on whether it carries a guard. The
exemption's own justification is about unguarded copies, so the condition and
its rationale disagreed. The result was inverted: guarded copies into a non-bare
face were exempt, while guarded copies into the bare face were enforced.

Consequently a guarded promote into the bare face required the caller to hold
`update` on the target type, and that same grant makes the target directly
editable by hand, which is exactly what the guard exists to prevent. No grant
permitted the promote and forbade the hand edit.

## Why this blocked a real deployment

A deployment that declares `bare_face: vastgesteld` makes the adopted version
the one every reader without a face address gets, including exports, feeds,
documents, CLI, MCP and Lua. The draft lives at `@concept`, and a guarded copy
promotes it. The control that adopted text changes only through that promote
could not be expressed.

## Fix

A guarded copy that MOVES content between two declared faces of one entity is
authorized by its guard:

```go
if guarded && plan.def.IsSameEntity() && plan.sourceTail != plan.targetTail {
    return nil
}
```

All three clauses are load-bearing, and code review caught that the first cut
had only the first two:

| Clause | Why |
| --- | --- |
| `guarded` | The security property itself. The target face was only a proxy for it, and the proxy inverted the rule. |
| `IsSameEntity()` | The caller chooses no target: `planCopy` pins `targetID` to the source and refuses a caller-supplied one. |
| `sourceTail != targetTail` | `IsSameEntity()` compares types, not faces, so it also admits `from: t` / `to: t` on a type with no faces at all. There is no guarded face there and no promote, so exempting it would hand out "mutate this entity in place, authorized by a permission noun". |

Unchanged: unguarded same-entity copies still need the ordinary write grant (the
revert protection), cross-entity copies stay fully enforced including the
per-edge authorization, and check (1) read-on-the-source stays unconditional so
a guard never becomes a way to publish a draft the caller cannot read.

Check (1) does **not** bound field-level disclosure — `PermitsReadFace` is a
row-and-face verdict. A guarded `fields: all` copy writes every property of the
target including ones the caller cannot see, and drops any property that exists
only on the target, because it is a full replace rather than a merge. Both are
now stated in the doc comment and in `docs/content-states.md`.

## Verification

Mutation-tested — removing any single clause fails a distinct test:

| Clause dropped | Test that fails |
| --- | --- |
| `guarded` | `TestCopy_IntoTheBareFaceNeedsUpdate` |
| `IsSameEntity()` | `TestCopy_GuardDoesNotOverruleACrossEntityWrite` |
| `sourceTail != targetTail` | `TestCopy_GuardDoesNotOverruleASameFaceCopy` |

Plus `TestCopy_GuardIsTheAuthorizationForASameEntityCopy` (guard suffices
without `update`; missing guard refused with `RuleKind: "copy-guard"`; read gate
consulted and enforced) and
`TestCopiesForSource_GuardedCopyIntoTheBareFaceIsOffered` (the affordance hint
must agree with the write, RULING 11).

The scoping tests assert *which* rule refused rather than that some refusal
happened, so they cannot pass under a blanket deny.

Full suite, `just lint`, `just arch-lint`, `just comment-lint` and `just
coverage-check` clean.
