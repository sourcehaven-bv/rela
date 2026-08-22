---
id: REV-TQEMN1
type: review-checklist
title: 'Review: Remove sidebar entity counts (badges, ACL-scoped counting path, docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full `just test` suite green (exit 0). `just lint` 0 issues; `just comment-lint`
no findings across 10178 comments; `just arch-lint` OK; `just lint-md` 0 issues;
`just docs-check` regenerates `docs/` byte-identical from the `docs-project/`
sources (confirms both mirrors are consistent). `just coverage-check` floors
pass at 77.4% total; `internal/dataentry` measures 81.0% and
`internal/apiwire/v1` 73.3%, both well clear of the 50% package floor.

**Comment findings.** No new findings introduced — the gate rule
(`commented-code`) is clean, and the diff removes comments rather than adding
them. No suppressions were needed, so none were added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None created — the review returned **zero critical and
zero significant findings**, so there was nothing meeting the bar for a
`review-response` entity. Verdict: "a clean, complete removal … nitpicks only;
ship it."

The reviewer independently verified the load-bearing property: the ACL read gate
is intact. `readGateFromContext` / `ReadQuery` are untouched,
`scopedSortedEntities` still resolves the gate (`api_v1.go:292`), and the gate
retains 9 other live consumers. `permitsNavEntry` and icon resolution are
behaviourally identical. It also confirmed the deleted tests were genuinely
count-only — the one non-count property asserted in
`TestACLSidebar_DenyAllZeroCounts` (ungated entries are never filtered) remains
pinned by `TestNavPermission_NonHolderFiltered`.

Two minor nits were raised and **both fixed** in commit 29cb3016 rather than
deferred:

1. `app.go` asserted a present-tense property ("the live sidebar serves no
counts at all") that nothing pinned — if someone re-added a badge the comment
would silently become a lie. Trimmed to the historical `#1043` rationale it
exists to record.
2. `navEntryToSidebarItem` was left as a method on `*viewsHandler` despite being
pure after the removal. Made a package-level function, which drops a needless
receiver and lets `nav_icon_test.go` stop spinning up a full `newTestAppV1`
fixture to test a `switch`.

A third observation (the `docs/acl-security.md:619` aggregate invariant still
naming "badge") was deliberately left as-is: it is forward-looking guidance for
the next person adding an aggregate, not a claim that a badge ships today.

Self-review: the diff contains no unrelated changes. Every non-count edit is a
comment corrected because the code it described was deleted.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (no `count` key on `/api/v1/_sidebar`) — **PASS**. Probed a live response
with a matching entity seeded:
`{"label":"All","href":"/list/all","icon":"list"}`. Field removed from the wire
type, so any reintroduction is a compile error.
- AC2 (no badge in the SPA; typecheck passes) — **PASS**. `vue-tsc` clean;
1824 frontend unit tests pass across 113 files.
- AC3 (no counting on the sidebar path) — **PASS**. `sidebarCounts` and its
three methods gone; verified by `go build ./...` plus a grep with no hits.
- AC4 (permission filtering + icons unchanged) — **PASS**. All
`TestNavPermission_*` and `nav_icon_test.go` green, and independently
re-verified by the reviewer.
- AC5 (no doc describes counts) — **PASS**. Grep across `docs/`,
`docs-project/`, `internal/`, `frontend/src` returns nothing; `docs-check`
confirms the mirrors agree.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind: chore`, not an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a chore)

Although no docs-checklist is required, user-facing docs were updated as part of
the change: `data-entry.md` (count-badge description), `acl-security.md` (the
sidebar-count gating paragraph and the config-filter performance caveat) and
`server-security.md` (two aggregate-gating lists), each in both `docs/` and the
`docs-project/entities/guides/` source.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1404

**CI status at review close:** Architecture, Cross-Compile (all six
platform/backend combinations), Fuzz, Comment lint, Lint Markdown, Vulnerability
Check and Analyze (actions) all green. Test / Lint / Frontend / E2E / Postgres
Backend / Analyze (go) were still executing when this checklist was closed —
every one of them has an exact local equivalent that was run and passed before
the PR was opened (`just test`, `just lint`, frontend `typecheck` + 1824 unit
tests, `just coverage-check`).

The "Rela Tickets" job failed on its first run for exactly one reason: this
checkbox was unchecked while CI was mid-flight, which its validator reports as
"Done review checklists cannot have unchecked items". Reproduced locally with
`rela validate --project tickets`, which named the same single finding. No code
or test failure was involved.
