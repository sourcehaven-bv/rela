---
id: REV-35RFK2
type: review-checklist
title: 'Review: Reject an unmatched verified principal''s writes (unmatched_principal policy key)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated checks

- [x] `just test` (full) — pass
- [x] `go test -race ./internal/acl ./internal/dataentry` — clean
- [x] `just lint` — 0 issues
- [x] `just lint-md` — 0 issues (docs edited at source)
- [x] `just arch-lint` — OK
- [x] `just coverage-check` — PASS (76.6%)
- [x] `generate-docs` diff — clean

## Design review (pre-implementation)

- [x] Ran before writing code; **drove the reject/provision split**
- [x] All findings addressed or deferred

The design review of the original combined (reject + provision) plan found 1
critical + 5 significant. It's why this ticket is reject-only:

| ID | Severity | Disposition |
|---|---|---|
| RR-BZQ049 | critical | **Addressed** for reject: enforced at the single `AuthorizeWrite` choke point (covers CRUD/sync/action/attachment); provision half parked |
| RR-9THKDO | significant | **Addressed**: guard from `a.jwtGate != nil` wiring state + load invariant requiring the lookup |
| RR-64WDUD | critical | **Deferred** (provision only — the minimal-grant vs. automations contradiction) |
| RR-9XBIJZ | significant | **Deferred** (provision only — re-stamp/read-gate rebuild) |

## Code review (post-implementation)

- [x] `cranky-code-reviewer` run against the branch diff
- [x] All findings addressed

**Zero critical/significant.** One minor + two nits, all addressed (RR-E12CN2):

- **minor:** docs claimed the 403 "leaks nothing" but the CRUD path returns
`rule_kind`/`reason` like every ACL denial (sync returns generic). Fixed the
docs to match — not the code, since the reason isn't sensitive and suppressing
it would break the uniform affordance-denial shape.
- **nit:** comment pinning "SetJWTGate must precede NewRouter" (the `jwtVerified`
snapshot invariant).
- **nit:** expanded the anti-bypass test comment to name the covered-by-
construction write paths (conflict-resolve calls `AuthorizeWrite` directly).

The reviewer independently verified, by tracing + running code: no
`Request.AuthorizeWrite` bypass; the flag is set on every write path and never
leaks to a background/cascade ctx; the load validation is complete and
fail-closed; `sync.Once` is copy-safe (pointer receivers, copylocks clean);
header/CLI/scheduler are untouched; ordering doesn't mask the unstamped check.

## Acceptance verification

AC1-AC8 each mapped to a named test (see IMPL-SJQJ5C). The anti-bypass test
(AC2) was **fault-injected** — disabling the reject branch flips CRUD + sync to
2xx, and the test catches each; the sync 200-under-injection proves it genuinely
reaches and is gated at authz, which was the design review's core concern.
