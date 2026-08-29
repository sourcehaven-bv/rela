---
id: REV-I3A8TM
type: review-checklist
title: 'Review: Extract dataentry theme/settings/palette cluster to appearanceHandler (App 104 → 92)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full suite; `-race` on dataentry; CI green except the Rela Tickets gate, resolved by this ticket's files landing on the branch)
- [x] Linters pass (golangci-lint 0 issues, plimsoll at 92, arch-lint, comment-lint clean across 11,075 comments)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, independent verification of the 11c02c1b..3c4d129b diff)
- [x] All critical/significant findings addressed (ZERO findings at any severity — reviewer's normalized-receiver diff proved every remaining line is a rename; the single substantive rewrite is a literal inline of the Cfg() accessor, same atomic-load count)

## Verification

- [x] Read path verified: handleAPIGetSettings keeps the identical listFromStoreByTypes → viewReader.Filter pair; handler holds no store.Store; ACL redaction tests pass
- [x] By-value handles verified safe: logo/palette/settings/viewReader assigned exactly once; palette reload mutates in place (Reresolve) rather than swapping the pointer
- [x] Wiring parity verified: NewApp + rebindApp both construct the handler; all 11 test-App call sites route through rebindApp
- [x] Method count independently verified: 92 production methods on App; appearanceHandler at 12

**Notes for the arc (out of scope here, log on TKT-N0IKN9/TKT-R68TV8):** stale
`// coverage-ignore: HTTP handlers tested via e2e tests` at
settings_handlers.go:316, and e2e/tests/settings.spec.ts:201 references the
deleted handlers_api.go. Reviewer also suggests migrating older handlers'
duplicated struct-literal construction onto the newAppearanceHandler(app)
constructor pattern.
