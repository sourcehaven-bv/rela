---
id: RR-QTCKCW
type: review-response
title: Plan's ++new++ sentinel premise is factually wrong; AC6 tests a non-existent risk
finding: |-
    The plan (and TKT-7K3BJF's acceptance criteria) treat the `++new++` STAGED_ID sentinel as a live value that must be prevented from reaching the server, with AC6 'the ++new++ sentinel never appears in any request URL or body' and a Security Considerations paragraph about it.

    This is false. `grep -rn 'STAGED_ID|isStaged' frontend/src/` excluding tests returns ONLY the definition site (`stagedEntity.ts:12,14-15`). Neither `STAGED_ID` nor `isStaged` is referenced by any production component. `DynamicForm.vue:1742` passes `:entity-id="entityId"` which is `props.entityId` — genuinely `undefined` in create mode, never the sentinel. The FileWidget's `!!entityId` check therefore already discriminates create from edit correctly, via undefined.

    Consequences: (a) AC6 is a test of a risk that does not exist, giving false assurance; (b) the plan's `canStage = ... && !entityId` is correct but for a different reason than stated; (c) worse, a reader could 'fix' this by INTRODUCING the sentinel into entityId to make the guard explicit, which would create the very leak the criterion guards against.

    Fix: drop AC6 and the sentinel paragraph, or restate accurately as 'create mode passes entityId=undefined; uploads only ever use the server-returned entity.id'. If STAGED_ID is genuinely dead code, that is a separate cleanup observation.
severity: significant
resolution: Corrected in both the ticket and PLAN-LCUIQO. The ticket's problem statement now says create mode passes `entityId === undefined` (DynamicForm.vue:1742) rather than the sentinel, with a parenthetical noting STAGED_ID has no production consumer. AC6 was rewritten from 'the ++new++ sentinel never appears in any request' to 'uploads use only the server-returned entity.id', which is the invariant that actually has value. The plan's Security Considerations paragraph now carries an explicit warning NOT to 'make the guard explicit' by assigning the sentinel into entityId, since that would manufacture the leak the old wording imagined.
status: addressed
---

## Finding

The plan asserts a security property about the `++new++` sentinel that is not
grounded in the code.

**Verified:**

```
grep -rn 'STAGED_ID|isStaged' frontend/src/ | grep -v '\.test\.'
frontend/src/components/forms/stagedEntity.ts:7   (comment)
frontend/src/components/forms/stagedEntity.ts:10  (comment)
frontend/src/components/forms/stagedEntity.ts:12  export const STAGED_ID = '++new++'
frontend/src/components/forms/stagedEntity.ts:14  export function isStaged(...)
frontend/src/components/forms/stagedEntity.ts:15
```

Definition site only. No production consumer.

`DynamicForm.vue:1742` binds `:entity-id="entityId"` → `props.entityId`, which
is `undefined` in create mode. So `FileWidget`'s existing `!!entityId` guard
distinguishes create from edit through `undefined`, not through a sentinel.

## Why this matters beyond pedantry

The dangerous direction is the "fix". A developer reading AC6 ("the sentinel
must never reach the server") could reasonably conclude the sentinel *is* in
`entityId` and make it explicit — introducing exactly the leak the criterion
purports to prevent. A security criterion that describes a non-existent
mechanism is worse than no criterion.

## Resolution

Restate as the true invariant: create mode passes `entityId === undefined`, and
post-create uploads use only the server-returned `entity.id`. Keep a test that
uploads are called with the returned id — that has real value — but drop the
sentinel framing.
