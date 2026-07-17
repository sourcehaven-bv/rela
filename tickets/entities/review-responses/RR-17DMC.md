---
id: RR-17DMC
type: review-response
title: history:purge permission is inert in the CLI-only surface — CLI has no ACL gate
finding: 'Both reviewers (cranky C1 = architect S2). The ticket says purge is ''gated on history:purge, mirror PermHistoryRead plumbing.'' But PermHistoryRead is enforced ONLY at the dataentry HTTP boundary (readGateFromContext(ctx).HoldsPermission). The CLI has NO per-invocation ACL gate: kong.go stamps every run as principal.SystemUser()/ToolCLI, cliServices exposes Store() directly, and the existing history/restore CLI commands do ZERO permission check — the CLI IS the trust boundary (shell + RELA_DATABASE_URL access = operator). Half-wiring a permission constant nothing enforces is worse than none: it creates a false belief purge is role-gated. Fix (both): OWN the reality — v1 CLI purge relies on the existing CLI full-trust model, SAME as `rela restore`/`rela db migrate` (operator shell = the boundary), state it plainly in docs. DEFINE PermHistoryPurge as the permission for a FUTURE dataentry/API purge surface (deferred v2), explicitly unused by the v1 CLI. Do NOT build an ACL path into the CLI (never done for any command; far larger than effort:m).'
severity: critical
resolution: 'Design revised: v1 CLI purge trust boundary is operator shell + RELA_DATABASE_URL (like rela db migrate), stated in docs; PermHistoryPurge defined only for a future API surface, unused by v1 CLI. No ACL path wired into the CLI. See revised design #4.'
status: addressed
---
