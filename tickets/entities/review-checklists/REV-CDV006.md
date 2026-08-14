---
id: REV-CDV006
type: review-checklist
title: 'Review: CalDAV deployment documentation (rela behind Pratique)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: documentation-only
ticket, no Go code)

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: no code to review — this ticket delivers
`docs/caldav.md` and `docs/caldav-clients.md`)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

## Verification

- [x] Each acceptance criterion tested — the guide was followed end-to-end
against a live Pratique instance with Apple Reminders (macOS 26.5.1) and
Thunderbird 153, and corrected where reality diverged from the first draft:
the `--allowed-origin` requirement for proxied deployments, the RFC 6764
`/.well-known/caldav` passthrough, and mandatory TLS for Reminders.
- [x] Test evidence documented — wire captures and per-client behaviour are
recorded in `docs/caldav-clients.md`, including what each client does with a
refused write.

## Documentation

- [x] Docs-checklist created and linked via `has-docs` (DOCS-CDV005)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CDV005

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1308
