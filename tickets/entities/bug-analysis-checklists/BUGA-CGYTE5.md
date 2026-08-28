---
id: BUGA-CGYTE5
type: bug-analysis-checklist
title: 'Analysis: acl audit A7 reports rela''s own built-in permissions (history:read) as dead, and suggests a fix that breaks a working grant'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced end-to-end against the built binary with a 12-line project (one
entity type, an `acl.yaml` granting only `history:read`, no `data-entry.yaml`).
Confirmed by A/B: reverting the fix restores the exact reported finding, and the
message matches the original report verbatim.

```
[low] role "admin" grants permission "history:read" which no
      role_relations.requires_permission references; the permission is dead
      fix: reference "history:read" in a requires_permission gate, or remove it
```

Conditions: no `data-entry.yaml` needed — this fires on a bare policy, which is
what distinguishes it from the sibling bug. Version 4d935eb2 (develop).

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Confirmed at source: `checkDeadPermissions` (`internal/aclaudit/tier_a.go:176`)
seeds `used` from `role_relations[].requires_permission` only.
`acl.PermHistoryRead` (`policy.go:44`) and `acl.PermHistoryReadRedacted`
(`policy.go:53`) gate **read** paths, so they can never appear in a
`requires_permission` — the single place A7 looks. Their own godoc prescribes
the grant shape A7 calls dead.

Severity note recorded during analysis: the emitted `Fix` is the real defect,
not the false positive. Both branches it offers ("reference it in a gate, or
remove it") break a working deployment. An advisory linter that emits a
destructive remediation is worse than one that stays quiet.

A6 was checked and is unaffected — `permissionGranted` asks the reverse question
(is a *gated* permission granted by some role), which built-ins don't perturb.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: expose `acl.BuiltinPermissions()` and seed `used` from it.
`internal/aclaudit` already imports `internal/acl`, so no new dependency and no
interface is needed — this is why the bug was split from its sibling and can
land independently.

Registration over a literal list is load-bearing: hardcoding the pair inside
aclaudit fixes today's symptom and re-breaks on the next global constant added,
which is precisely the why5 failure mode.

Regression test: `AM-acl-builtin-permissions-audit-exempt` — table-driven over
`acl.BuiltinPermissions()`, asserting the list is non-empty AND that each
constant produces no A7 finding, plus a guard that a real dead permission
granted alongside a built-in is still reported (so the fix cannot degenerate
into suppressing the rule).

Related areas: the same used-set incompleteness affects `data-entry.yaml` UI
gates. Filed separately as [[BUG-919PM6]] because it needs a consumer-side
interface and a CLI adapter — different fix, different size.
