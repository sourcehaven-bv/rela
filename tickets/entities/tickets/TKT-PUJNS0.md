---
id: TKT-PUJNS0
type: ticket
title: 'lua: add ACL-gating test for rela.md.entity_refs (IB finding on #1197)'
kind: test
priority: medium
effort: xs
status: done
---

## Summary

IB-review finding on PR #1197 (rela#1199, severity Low, CONTROL-8-29).

The RR-ZA452J critical defect — `rela.md.entity_refs` used
`context.Background()` instead of `callerCtx()`, so with a policy configured it
resolved no principal and returned an **empty map for every user** — was fixed
in #1197, but unlike the other two critical fixes in that PR it shipped
**without a dedicated test** exercising it under a configured ACL policy. That
undercut the PR's own claim that "every gate now fails a test when removed", and
left the regression free to return unnoticed.

## What was added

`TestScriptReads_EntityRefsGated` in `internal/lua/aclreads_test.go`, reusing
the existing `newACLWorld` fixture (real `acl.Declarative` +
`affordances.PolicyResolver` over a memstore).

It asserts **both** halves, because asserting only the second would let the
original bug back in disguised as a pass:

1. Entities the principal MAY read are **present** — this is what the
original defect broke (everything fell closed).
2. Entities the principal may NOT read are **absent** — the gate applies.

## Mutation verification

Both directions confirmed to fail, each with a diagnostic naming the real cause:

- Restoring `ctx := context.Background()` (the original RR-ZA452J defect) →
fails with `refs=` empty and the message "the binding resolved no principal and
fell closed for everyone".
- Routing the listing through the raw `WritePrepStore` instead of the gated
reader → fails with `refs=P-1,SEC-1,TKT-1` and "a hidden entity leaked into the
ref map".

No production code changed — `internal/lua/markdown.go` is untouched; this is
purely the missing test.

## References

- rela#1199, PR #1197, RR-ZA452J, TKT-ZF2DTV
