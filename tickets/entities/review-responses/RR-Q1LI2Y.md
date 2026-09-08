---
id: RR-Q1LI2Y
type: review-response
title: internal/docs/resolvers_acl.go is a second, hand-copied grant matcher the survey missed
finding: |-
    VERIFIED. dn37j2-plan.md §1.1 presents a table headed 'Its consumers, enumerated exhaustively' with eight rows. It is not exhaustive: internal/docs/resolvers_acl.go:116-136 reimplements the match.

      func grantsList(list []string, target string) bool {
          for _, t := range list { if t == "*" || t == target { return true } }
          return false
      }

    Its own comment says it 'replicate[s] the policy's wildcard-or-exact match over the exported RoleDef grant lists (the policy's own helpers are unexported)'. roleGrantsVerb (:94-99) dispatches read -> grantsList(role.Read, typ), else a local grantsVerb copy (:116-127). It reads dr.policy.Roles[rn] RAW and UN-CLAMPED (:67) — i.e. it also bypasses the roleFor ceiling clamp point, though as a docs generator that is arguably in scope.

    Two consequences:

    1. WRITE GRANTS (kept joined per binding §8): `update: ["page@draft"]` renders in the `rela docs` role matrix as 'cannot update page' — a FALSE ALL-CLEAR in an operator-facing security document, disagreeing with what acl.grantsVerb actually permits.
    2. RoleDef.Worlds: the matrix has no worlds axis and would silently omit the entire new grant kind.

    This is exactly the blind spot dn37j2-plan.md §9 predicted (I grepped acl/aclmap/aclaudit/affordances and did not grep internal/docs).

    FIX: export the canonical predicates from internal/acl and delete the copy. A duplicated authz matcher is not acceptable while the grammar changes under it.
severity: critical
status: open
---
