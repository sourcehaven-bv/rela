---
id: REV-648DHW
type: review-checklist
title: 'Review: Reject reserved system:* principals at the API boundary (system:scheduler impersonation)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` — full suite green, race detector on)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Coverage maintained (`just coverage-check` — PASS, total 78.1%)
- [x] `just arch-lint` — no warnings; `just plimsoll` (god-object lint) — clean

## Code Review

- [x] Ran the cranky-code-reviewer agent over the full diff
- [x] All critical review-responses addressed — **none were raised.** The
reviewer attempted to break the guard and could not; it independently traced and
cleared every bypass candidate (webhook, CalDAV incl. its unauthenticated
discovery routes, MCP, the off-API split, `principal_property` substitution,
case sensitivity, unicode, the 256-rune cap, and the legitimate in-process
stampers).
- [x] All significant review-responses addressed — 3 of 3
- [x] Minor/nit addressed — 3 of 3 (none deferred or wont-fixed)
- [x] Self-reviewed the diff for unrelated changes — `git status` shows only the
intended files; the docs edits are in `docs-project/` sources with `docs/`
regenerated, no unrelated churn

**Review Responses:**

| ID | Severity | Status | Summary |
|---|---|---|---|
| RR-NQK412 | significant | addressed | `IsReserved` was not control-char safe, so its standalone-correctness claim was false — safe by ordering, not construction |
| RR-OJRCNY | significant | addressed | JWT gate classified its log line on the raw subject, degrading the security WARN to INFO for the variant an attacker would send |
| RR-O4VZW0 | significant | addressed | No test covered the sanitize/`IsReserved` control-char interaction |
| RR-I5S4NK | minor | addressed | 403 body reflected the attacker-supplied name unnecessarily |
| RR-ZQHPHC | nit | addressed | `slog.Warn` → `slog.WarnContext` in provision.go |
| RR-14CXHG | nit | addressed | Docs table implied a meaningful 403-vs-401 asymmetry |

**The two findings that mattered were the same root cause seen from two sides:**
`IsReserved` did `TrimSpace` + `HasPrefix`, so `"\x01system:scheduler"` was not
reserved to it — while `sanitizeUser` turns that exact input into
`"system:scheduler"`. Requests were still denied (sanitization ran first), so
this was never an authorization hole; it was a *detection* hole plus a latent
fragility for any future caller passing a raw value — and `provision.go` already
does. Fixed on both axes: `IsReserved` now strips leading C0/DEL as well as
whitespace (safe by construction), and `verifiedPrincipal` returns a typed
`rejectReason` so the gate cannot re-derive a second, disagreeing answer.

The reviewer's leverage item was also taken: a guard test in `internal/entity`
now pins that `idPattern` rejects `:`. That property is what stops
`resolvePrincipalEntity` — which substitutes an entity ID as the acting
principal *after* the boundary check — from ever producing `system:scheduler`.
It was load-bearing and previously unwritten.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-NVXPNJ)

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| 1 — header rejected, no fall-through to `unknown` | **PASS** | Live 403 + `TestStampAuditPrincipal_RejectsReservedHeaderUser`, `..._ReservedRejectionIsNotADowngrade` |
| 2 — `$RELA_DATAENTRY_USER` rejected | **PASS** | `TestStampAuditPrincipal_RejectsReservedEnvUser` |
| 3 — validly signed JWT rejected (survives gate re-stamp) | **PASS** | `TestRequireVerifiedJWT_RejectsReservedSubject`; mutation-confirmed |
| 4 — any `system:`-prefixed user | **PASS** | 4-value table incl. bare `system:`, plus 7 `IsReserved` noise rows |
| 5 — remote MCP endpoint | **PASS** | `TestRequireVerifiedJWT_ReservedSubjectRejectedOnMCPPath` |
| 6 — scheduler in-process unaffected | **PASS** | `internal/scheduler` suite green |
| 7 — provisioning unaffected; no reserved stub | **PASS** | `TestProvision_ReservedSubjectNeverProvisioned`, `TestMaybeProvision_RefusesReservedSubjectDirectly` |
| 8 — rejection logged with source + remote addr | **PASS** | Live: `level=WARN msg="rejected reserved principal asserted by request" user=… path=… remote_addr=…`; plus the WARN-vs-INFO discrimination tests added for RR-OJRCNY |

**The vulnerability was reproduced before it was fixed.** Against a build with
`IsReserved` forced to `return false`, a spoofed `X-Remote-User:
system:scheduler` read a ticket that the ordinary user (`everyone: read: []`)
could not see. The same request on the fixed build returns 403. That is the
evidence this ticket rests on — not the tests alone.

Non-regression, verified live: `alice` → 200, `System:Scheduler` → 200
(case-sensitive by design), `my-system:scheduler` → 200, and `GET /` carrying
the forged header → 200 with the SPA served (the RR-T15E constraint).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (3 guides, incl. an upgrade note for
deployments whose IdP issues `system:`-shaped subjects)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9HD96R

## Final Checks

- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the rule is carried by godoc at the
place a future entry point will encounter it (`IsReserved`), by an
anti-documentation note on `sanitizeUser` warning off the tempting-but-wrong
location, and by a guard test in `internal/entity` protecting the unwritten
`idPattern` dependency
- [x] Commit message explains the why, not just what

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: committed to
  the branch `tkt-9pcl7d-reserved-system-principals`; opening the PR is the
  user's call, since pushing is an outward-facing action they have not asked
  for. Run `/pr` to open it.)
- [x] ~~All CI checks pass~~ (N/A until the PR exists; the full local
  equivalent — `go test ./...`, `just lint`, `just arch-lint`, `just plimsoll`,
  `just coverage-check` — is green, see Automated Checks above)
- [x] ~~PR URL documented below~~ (N/A until the PR exists)

**PR:** not yet opened — branch `tkt-9pcl7d-reserved-system-principals`
