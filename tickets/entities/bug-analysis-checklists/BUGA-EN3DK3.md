---
id: BUGA-EN3DK3
type: bug-analysis-checklist
title: 'Analysis: acl audit A7 cannot see data-entry.yaml permission gates, so every UI-gating permission is reported dead'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced end-to-end against the built binary: a project whose `acl.yaml`
grants `report:sales` and whose `data-entry.yaml` gates a document with it. A7
reported the permission dead once per granting role while the gate demonstrably
worked. Reverting the fix restores the finding; applying it yields a clean
audit.

Conditions: requires BOTH files present — the permission must be granted in
`acl.yaml` and referenced only from `data-entry.yaml`. That two-file setup is
what separates this from the sibling bug, which fires on a bare policy.

Also reproduced for a **nested** navigation entry (a `group:` with `items:`),
which a flat walk of the navigation list would miss.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

`checkDeadPermissions` seeds `used` from `role_relations[].requires_permission`
alone. Four further consumers were confirmed at source in
`internal/dataentryconfig/config.go` — documents (`:668`), dashboard cards
(`:773`), navigation (`:896`), commands (`:964`) — none reachable from
`acl.Policy`.

The systemic finding: aclaudit's dependency posture (declare a narrow
consumer-side view of what you need, let the caller supply it) was applied to
the metamodel via `MetamodelReader` but never carried to permission consumers.
A7 was classed as a pure-policy Tier-A check; it is not, because it asserts a
whole-system fact while seeing one file.

Note the detection is not wrong in form — the permission genuinely has no
`requires_permission`. Only the conclusion ("dead") is false.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: a narrow `aclaudit.PermissionConsumer` (`UsedPermissions() []string`)
with the concrete adapter consumer-side in `internal/cli`, mirroring
`MetamodelReader` / `metamodelReader` exactly. aclaudit keeps depending only on
`internal/acl`; arch-lint confirmed clean.

Load-bearing design decision: **nil must not mean "run anyway."** The
`MetamodelReader` precedent is that nil silently drops checks — safe there,
because those checks cannot be formed without a metamodel. For permissions, nil
means "I could not see the UI gates," and running A7 regardless reproduces this
exact false positive. So nil suppresses A7 instead.

Open question resolved during implementation: `rela acl audit` runs against a
project dir, so `data-entry.yaml` is reachable. A missing file is *complete*
information (no gates exist) and yields an empty consumer; only an unreadable or
unparseable file yields nil and suppresses the check. The adapter deliberately
does not validate — the audit's subject is `acl.yaml`, and an invalid data-entry
config should not cost an operator their ACL findings.

Regression test: `AM-acl-audit-permission-consumers-complete` — per-surface
coverage (so dropping one surface from the adapter fails its own subtest) plus
the nil-consumer fail-safe. Mutation-tested: deleting the nav recursion fails
only the nested-navigation subtest; deleting the dashboard loop fails only the
dashboard subtest.

Related areas: the sibling [[BUG-NRCJ9E]] covers rela's own built-in
permissions, fixable inside `internal/acl` alone. Landed first; this builds on
its plumbing.
