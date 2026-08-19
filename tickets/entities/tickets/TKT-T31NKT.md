---
id: TKT-T31NKT
type: ticket
title: Gate the membership relation against ACL self-promotion
kind: enhancement
priority: critical
effort: m
status: done
---

Security gate for shipping content states — design doc §12.1
(`.ignored/pointer-design.md`).

`internal/acl/policy.go:324-351` documents a live self-promotion path: the
membership relation carries no `requires_permission` by default, so anyone who
can write the source type can write `alice --member-of--> admins` and
self-assign any role in `assignments:`. Pre-existing; with world/public read in
the picture it becomes the mechanism for leaking unpublished content.

**Note:** `A1-ungated-membership` already exists at severity High
(`aclaudit/tier_a.go:55`) and trips `--fail-on=high`/`--exit-code` — the audit
side is done. The gap is that enforcement is advisory: a server loads and serves
a policy with the hole open unless someone runs the audit. Verified 2026-08-19:
no `world:` grant syntax exists in `internal/acl` yet, so the refusal condition
is unevaluable until Step 3 (TKT-DN37J2).

**Scope now (this ticket):**

1. Extract the A1 predicate from `aclaudit/tier_a.go` into `internal/acl` as a function on `Policy`; aclaudit calls it (aclaudit already imports acl — direction fine). Behaviour identical; tests move with it. Prevents validator/auditor drift later.
2. rela-server logs the ungated-membership condition as a prominent startup warning when loading a policy with assignments. Warning, not refusal — unconditional refusal would break existing trusted-team deployments (out of scope).
3. Docs: record the coming deployment requirement (world grants + ungated membership = load refusal) in the acl-security docs.

**Deferred to TKT-DN37J2 (acceptance criterion there, NOT built here):**

4. The `Policy.Validate` refusal: policy grants read on a non-default world AND membership ungated per the shared predicate → hard load error naming the fix. Written against Step 3's real grant representation. Do NOT build a latent always-false `hasNonDefaultWorldGrant` hook now — untestable dead code that pre-commits Step 3's data model.

Not a fail-closed degrade: inert world grants = a public outage
indistinguishable from an ACL bug. Consistent with worlds' mandatory
`otherwise:` validated at load. Blocks *shipping* pointers, not starting them.
