---
id: REV-BZVSM0
type: review-checklist
title: 'Review: Realtime 3/7: SPA channels'
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test passes on the touched packages of each slice, with build and vet on the default, `postgres` and `sqlite` tags; frontend unit tests (4616), rela-components story tests and the channels and presence e2e specs pass
- [x] Lint clean (`just lint`): golangci-lint, `just arch-lint`, `just plimsoll`, the frontend lint (no errors) and the rela-components checks including purity
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): coverage floors pass in CI on PRs #1937 and #1938; the T7c PR runs the same gate in CI

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on each slice (T7a, T7b, T7c)
- [x] All critical review-responses addressed: the two critical design-review findings (restricted refs, sanitizer allowlist) are addressed; the code reviews raised none
- [x] All significant review-responses addressed: all are addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 95 review-response entities linked via
`has-review-response`: 29 from the design review and 22, 24 and 20 from the
reviews of T7a, T7b and T7c. Open findings: none. 94 are addressed; one nit (a
multi-type picker search) is wont-fix because it needs a server API change.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-9UAJLA and plan section 15
- [x] Test evidence documented in implementation checklist: IMPL-F2OZVE

**Acceptance Status:**
- Channel schema endpoint, `channels:` config and validation: PASS, `TestChannelSchema_*`, `TestChannelsPresentation_Validate`, OpenAPI golden
- Per-message actions, edit preserving hidden and restricted props: PASS, `internal/realtime` msgops tests, `ChannelsPanel.test.ts`
- Refresh on reload, wire drift check: PASS, dataentry reload tests, `TestChannelWireGolden`, `wireFields.test.ts`
- Composables, cursor and paging, sanitizer: PASS, `useChannel.test.ts`, `useEntityChannels.test.ts`, markdown tests
- Library components, restricted ref placeholder, controls from actions: PASS, channel story play tests
- Panel on the entity page, live updates, ACL and faces: PASS, `ChannelsPanel.test.ts`, `e2e/tests/channels.spec.ts`

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime and GUIDE-data-entry, regenerated into `docs/`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GLSRIM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PRs #1937 and #1938 merged; the T7c PR carries the last slice and is monitored after the ticket reaches `done`; see TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
