---
id: RR-YALMJ1
type: review-response
title: 'aclaudit: B1/B8 are the checks that break, not A9/A7; and B1 fires in PR-A not PR-B'
finding: |-
    VERIFIED. dn37j2-plan.md §1.5 and §2.3 point at the wrong checks.

    IMMUNE (plan wrongly implicated / cited): A9-wildcard-write (tier_a.go:225-242) only calls hasWildcardWrite, testing `== "*"`; A7-dead-permission (tier_a.go:180-220) iterates Permissions, never types. Neither is affected by type@face.

    ACTUALLY BREAKS:
    - B1 checkUndeclaredEntityTypes (tier_b.go:29-59) consumes verbLists (aclaudit.go:233-240) INCLUDING create/update/delete. m.HasEntityType("page@draft") => false => HIGH finding. cli/acl.go gates --exit-code on High, so EVERY state-shaped write grant FAILS CI. The plan lists B1 but only for the READ arm; the WRITE arm is the one that fires under the binding §8 decision to keep write grants joined. CONSEQUENCE: this must land in PR-A, not PR-B, because PR-A introduces the syntax and must be green alone.
    - B8 ceilingTypeFindings (aclaudit/ceiling.go:231-253): an 8-axis table doing the same `t == "*" || m.HasEntityType(t)` check at :240, emitting High. §2.3's PR-A item cites aclaudit/ceiling.go:125-135 — that is the A12 unreachableTargets table, NOT B8. WRONG CITATION in the plan; correct it to ceiling.go:231-238.
    - A12 (ceiling.go:120-143) compares by exact string, so a baseline closing bare `page` is not seen as closing `page@draft` => Low-severity false 'reopens nothing'.

    ALSO: B1 shares a `seen` dedupe map across all four verbs (tier_b.go:33,37), so a bogus string in create suppresses a genuine finding for the same string in read.

    ALSO for RoleDef.Worlds: verbLists (aclaudit.go:233-240) and affordanceTypeKeys (aclaudit.go:256-283) hard-code their field lists non-reflectively — a new field is invisible with NO compile error. And A10's whitespace scan (tier_a.go:247-281) does not cover world names, so `worlds: [" prod"]` is silently inert, the exact failure class A10 exists for.
severity: significant
status: open
---
