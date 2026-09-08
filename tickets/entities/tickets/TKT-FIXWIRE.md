---
id: TKT-FIXWIRE
type: ticket
title: appbuildtest.New hand-copies the entitymanager.Deps literal instead of calling the real builder
kind: test
priority: medium
effort: m
status: backlog
---

**The test fixture and the composition root maintain two independent
`entitymanager.Deps` literals, and nothing derives one from the other.**

`appbuild.buildEntityManager` (`appbuild.go:~1270`) builds the production
`Deps`. `appbuildtest.New` (`appbuildtest/fixture.go:~190-215`) builds its own,
with a comment saying it mirrors the production wiring. It is a hand copy.

## Why this is a real defect and not a style complaint

This is exactly how the `Copy*` deps came to be unwired in **both** places at
once (found during TKT-WRLDAPI item 5, 2026-08-24):

- `appbuild` set `TransitionGuard` but none of `CopyGuard` / `CopyReadGate` /
  `CopyVisibility`, so **every promote/publish returned 403 on every real
  deployment**.
- `appbuildtest` had the identical gap, so **every test using the fixture ran
  without a capability real deployments have**.

The two lists were wrong in the same way, so they agreed with each other — and
a test suite that agrees with a broken production wiring reports green. The
defect was found only when an end-to-end test was written that drove a
composition-root-built manager and asserted an OUTCOME (a guarded copy actually
running), not that the deps were non-nil.

This is the **paired-list corollary** to RULING 15, in the wiring rather than
in a guard: an implicit list that must be kept in sync with another list, where
nothing derives one from the other. See `.ignored/ARCHITECT-RULINGS.md`.

## The interim state is a comment, which is the thing that does not work

Item 5 wired both sites and added a comment in the fixture naming the hazard.
That is strictly better than nothing and strictly not a fix: **a comment saying
"keep this in sync" documents an invariant it cannot enforce**, and the next
person to add a dep will not read it. Same species as the `// mirrors the
production wiring` comment that was already there while it did not.

## The fix

Have the fixture call the real builder. The signatures differ, so this is a
genuine change rather than a rename — the fixture supplies test doubles
(`NopACL`, an in-memory store) where production supplies real collaborators, so
the builder needs a seam that accepts both.

Failing that, derive the check rather than the value: a reflection test
asserting every field of `entitymanager.Deps` that production sets is also set
by the fixture, or explicitly listed as deliberately-nil with a reason. That is
the same shape as `TestCopyDef_UnmarshalCoversEveryField` and
`assertEveryWorldCapableRouteIsScanned`, both of which converted a hand-list
into a checked one.

## Note on the current nil fields

`CopyReadGate` / `CopyVisibility` being nil in the fixture is **deliberate and
correct** — it is the no-policy posture, matching the fixture's `NopACL` default
and its raw reads elsewhere. Do not "fix" those to non-nil. The defect is the
absence of a mechanism that would have caught `CopyGuard` being nil, not the
nil-ness of these two.

Related: [[TKT-WRLDAPI]].
