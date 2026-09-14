---
id: copy-guard-authorization-tests
type: automated-measure
title: Copy guard-is-the-authorization tests across write path and affordance surface
description: Mutation-tested coverage of the three-clause copy exemption rule (guard present, same entity, source face != target face). Removing any single clause fails a distinct test, so no clause can be dropped silently. The scoping tests assert WHICH rule refused rather than that some refusal happened, so they cannot pass under a blanket read-only deny. Also pins the affordance surface, where the Allowed hint must agree with the write.
kind: test
location: internal/entitymanager/copy_authz_test.go, internal/entitymanager/copylist_test.go
status: active
---

## What this catches

The copy write check exempts a copy from the ordinary `update` / `create`
requirement when it is guarded AND moves content between two declared faces of
one entity. Every shorter spelling of that rule admits or excludes a case the
reasoning never covered — which is BUG-J2E6VJ and, separately, the hole its
first fix opened.

Each clause is pinned by its own test, verified by mutation:

| Clause dropped from the condition | Test that fails |
| --- | --- |
| `Guard.Permission != ""` | `TestCopy_IntoTheBareFaceNeedsUpdate` |
| `IsSameEntity()` | `TestCopy_GuardDoesNotOverruleACrossEntityWrite` |
| `sourceTail != targetTail` | `TestCopy_GuardDoesNotOverruleASameFaceCopy` |

Plus the positive cases in `TestCopy_GuardIsTheAuthorizationForASameEntityCopy`:
the guard alone suffices under a read-only ACL; a missing guard is refused with
`RuleKind: "copy-guard"` even under an allow-all ACL; and the read gate is
consulted (the double records the call, because `ErrCopySourceMissing` is also
what an absent source produces).

## Two test-design rules this measure encodes

**Assert which rule refused.** `acl.ReadOnlyACL` denies every write with one
fixed decision, so a test asserting only "some `*acl.ForbiddenError`" passes
whether or not the scoping under test works. The cross-entity test uses
`createOnlyACL` against an existing target and asserts `RuleKind == "test"`, so
only check (3) reaching the ACL satisfies it.

**A face-crossing fixture for the cross-entity case.** A cross-entity copy into
a *new* faceless entity has `sourceTail == targetTail`, so it would be refused
by the face clause regardless and the `IsSameEntity()` clause would go
unexercised. The fixture is `from: ticket` / `to: page@published` so that clause
is the only thing refusing it.

`TestCopiesForSource_GuardedCopyIntoTheBareFaceIsOffered` covers the affordance
surface, where RULING 11 requires the hint and the write to agree.
