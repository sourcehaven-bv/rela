---
id: REV-UQF1XK
type: review-checklist
title: 'Review: Timed calendar-feed events from datetime sources (DTSTART with time, datetime start/end range)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; `just ci` passed end to end)
- [x] Lint clean (`golangci-lint` 0 issues after fixing gofmt/lll/shadowed-min; `just arch-lint` clean)
- [x] Coverage maintained (`just ci` includes coverage-check — passed)

## Code Review

- [x] Run `/code-review` command (invoked cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none — 0 critical/significant found; reviewer called the commit "clean, disciplined")
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes (only calfeed/feed/validation source + tests + docs; build artifacts gitignored)

**Review Responses:** 1 finding (RR-TPD401, minor — 3 stale doc comments),
addressed. The reviewer confirmed-correct all load-bearing behavior: byte-identical
all-day render (ETags stable), correct `Timed` zero-value default, mismatch
validation avoids double-reporting, JSON backward-compat, provider `Timed` keyed
on the start property. Design-review findings (6, 2 critical) were all addressed
during implementation — see the ticket's has-review-response links.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-S6IND8)

**Acceptance Status:** all PASS — evidence in IMPL-S6IND8:
- AC1 datetime `date:` accepted — validate_feeds_test valid case. PASS
- AC2 start/end kind mismatch rejected (both directions) — validate_feeds_test. PASS
- AC3 timed DTSTART (no VALUE=DATE) + date source still all-day — ical_test + live curl. PASS
- AC4 timed-range DTEND (verbatim) — ical_test TestRenderEvent_TimedRange. PASS
- AC5 JSON timed shape (start/end RFC3339, date empty, allDay=false) — json_test + live. PASS
- AC6 provider datetime source → Timed + time-of-day preserved — feed_provider_test. PASS
- AC7 handler ICS e2e for a datetime feed — feed_handler_test + live curl. PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updated inline — feed sections in the data-entry + metamodel source guides, regenerated; a 2-section doc update doesn't warrant a separate docs-checklist)
- [x] User-facing documentation updated (GUIDE-data-entry.md feed source table + events section; GUIDE-metamodel.md datetime feed note; regenerated to docs/*.md)
- [x] ~~Docs-checklist marked as done~~ (N/A per above)

## Final Checks

- [x] Commit message explains the why, not just what (feature + doc-fix + lint-fix commits, each with rationale)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending: branch is stacked on the datetime PR #1131; PR base to be decided -->
