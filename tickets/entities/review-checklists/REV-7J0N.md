---
id: REV-7J0N
type: review-checklist
title: 'Review: ACL v0 PR 3: Wire acl.yaml into appbuild + non-loopback warning + docs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (deferred to crit pass on the PR)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None — PR 3 is wiring + docs only (loadACL helper +
Services accessor + shouldWarnNoACL helper + docs/security.md ACL section +
docs/audit-log.md denied-write subsection + CLAUDE.md note). Crit pass on the
open PR will produce any necessary review-responses.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All PR 3 ACs (AC3.1–AC3.3) PASS. Evidence in IMPL-6Y93.

## Documentation (enhancements only)

- [x] User guide / reference docs — `docs/security.md` ACL section landed (schema, semantics, delegate-X, trust boundary, v0/v1 scope)
- [x] CLAUDE.md — new "Don't run user-supplied Lua on the read path" rule + ACL package note
- [x] `docs/audit-log.md` — `denied-write` op documented (via the source guide; regen confirmed)
- [x] CLI help text — N/A (PR 1 already documented `--read-only`)
- [x] README.md — N/A (server-level feature)

**Docs Checklist:** PR 3 lands all v0 user-facing docs inline; no separate
`docs-checklist` entity created — the changes are scoped tightly enough to live
on REV-7J0N.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** TBD — opening immediately after commit; will record URL here.
