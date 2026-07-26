---
id: REV-TXKHHZ
type: review-checklist
title: 'Review: fail-closed data-entry script read wiring'
status: done
---

## Automated checks

- [x] `go build ./...` clean
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — no warnings
- [x] `just plimsoll` — pass
- [x] Affected suites pass (dataentry, visibility, appbuild, lua, scheduler)

## Code review

- [x] `/code-review` run via cranky-code-reviewer.
- [x] Findings triaged, and two were **rejected after verification** rather than applied blindly:
  - "there are two consumers, not three" — refuted: `app.go:628` passes `app.luaWriteDeps` as a closure into `newDocumentService`, so `export_render` is a third consumer. The reviewer's grep missed the closure form.
  - "the `store.Store` assertion is unreachable dead weight" — refuted by probe: the assertion exists to catch the *fail-open* case (raw store returned), and it fires exactly then, as the mutation run confirmed.
- [x] Accepted findings addressed: RR-8JNKWC (critical), RR-DZ7H2S, RR-R38TA9.
- [x] No open critical or significant review responses.

## Verification

- [x] Security property mutation-tested, not merely asserted.
- [x] Policy-less regression risk explicitly pinned.
- [x] Local `docscapture` failures investigated to root cause (`frontend/dist/` absent) and proven pre-existing via a stashed-tree control, rather than assumed environmental.
- [x] PR opened: https://github.com/sourcehaven-bv/rela/pull/1204
