---
id: BUGA-C1PVEC
type: bug-analysis-checklist
title: 'Bug Analysis: Guarded copy into the bare face is refused unless the caller can already write the target'
status: done
---

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced as a unit test rather than a running deployment: a `page` type with
`bare_face: draft`, a guarded copy `page@published → page@draft`, and a
read-only ACL. `CopyState` returned `*acl.ForbiddenError` ("this rela instance
is configured read-only") where the guard alone should have sufficed. Confirmed
on `develop` at `e4cbda51`, default (fsstore/memstore) build.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded as `why1`–`why5` on the bug. In short: the same-entity exemption in
`authorizeCopy` tested the target face (`plan.targetTail != ""`) rather than the
guard (`plan.def.Guard.Permission != ""`). The load rule "a non-bare target must
declare a guard" makes the two coincide in one direction only, so the proxy read
as an equivalence and no test separated them.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: key the exemption on the guard, keeping `IsSameEntity()` as the condition
that makes it safe (the operator, not the caller, chose the target).

Related areas checked: `copylist.go` computes copy affordances by running the
real authorization path rather than re-deriving it, so it inherits the fix; a
test was added to pin that. Grepped every non-test use of `targetTail` and
`IsSameEntity` — no other site re-derived the rule. `docs/content-states.md`
stated the old rule in prose and was updated.
