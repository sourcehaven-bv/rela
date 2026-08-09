---
id: REV-3FYN9I
type: review-checklist
title: 'Review: Permission-based dashboard card filtering (UX: hide cards a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test -race ./internal/...` all pass. `just lint` 0 issues. `just arch-lint`
OK. `just plimsoll` OK. `just coverage-check` 77.3% (both thresholds satisfied —
up from 76.9% pre-change). Frontend: 1539 tests pass, `npm run lint` 0 errors,
`npm run build` clean (typecheck included — it catches test files that `npm run
typecheck` misses).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-9AECGP (critical), RR-NGPLP0 (significant), RR-XL4MSU
(minor), RR-2NMCA3 (minor), RR-QZORXX (nit) — all `addressed`.

The reviewer found a genuine **critical** bug I missed: `getDashboard()` sat in
the boot `Promise.all`, and since `doLoad` re-throws and `App.vue` turns a
rejected load into the full-screen error state, any `/_dashboard` failure took
down the *whole app* — sidebar, lists, forms — for a UX filter most deployments
never exercise. The realistic trigger is a newer SPA against an older server.
Fixed by settling the fetch to `undefined`; pinned by a store test that asserts
the boot succeeds and only the dashboard degrades.

The **significant** finding was the same gap from the other side: no route
probe, so deleting the `mux.HandleFunc` line left the entire package suite green
— every handler test bypasses the mux. Both are now covered.

Design-review findings from the planning phase (RR-PZKLVV, RR-QAEM5Z, RR-CLZB5I,
RR-TIO1XP) were all addressed before implementation started.

**Not accepted as findings:** none. Two things the reviewer explicitly checked
and cleared are worth recording — nothing downstream consumes the filtered card
list for enforcement (so "presentation only" holds), and `/_config` genuinely
still serves `dashboard:` verbatim (so the TKT-M1AX6P reversion is not being
quietly undone).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS — full evidence table in IMPL-XJCM01.

| AC | Status | Evidence |
|---|---|---|
| 1 ungated always shown | PASS | live: alice/bob/no-ACL; `_UngatedCardsAlwaysShown` |
| 2 Declarative gating | PASS | live: alice 3 cards, bob 2, order preserved; `_HolderSeesGatedCard`, `_NonHolderFiltered` |
| 3 NopACL + ReadOnlyACL show | PASS | live: `--read-only` and no-`acl.yaml` both show the gated card; `_ReadOnlyArmIsExplicit` canary mutation-verified |
| 4 nil / unknown ACL hides | PASS | `_NilACLHides`, `_UnknownACLHides` |
| 5 presentation-only | PASS | live BOTH directions: bob card hidden yet reads 1 row; carol card shown yet reads 0 rows |
| 6 `/_config` unfiltered | PASS | live: byte-identical alice vs bob, still carries "Audit Log" |
| 7 empty ⇒ 200 `[]` | PASS | live: all three causes → `[]` not `null`; UI renders the empty state |

Verified in the real UI (Chrome) against servers pinned to different identities,
and re-verified after the review fixes — including that the `cardKey` change
binds the right data to the right tile (count + breakdown cards both correct).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D7W18R

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One incidental fix worth flagging: `internal/dataentry/CLAUDE.md` named a
read-only canary `TestNavPermission_ReadOnlyHides` that does not exist, and
whose name asserts the *opposite* of the pinned behavior. Corrected.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending: not yet requested by the user -->
