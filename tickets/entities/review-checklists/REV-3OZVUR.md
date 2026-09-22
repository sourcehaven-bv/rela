---
id: REV-3OZVUR
type: review-checklist
title: 'Review: Replace command open/reveal launcher with an ACL-gated HTTP download'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: full Go suite green; `-race` clean on `internal/dataentry` (466s) and
`internal/migration`; `golangci-lint ./...` 0 issues; `just arch-lint`, `just
plimsoll`, `just comment-lint` all clean. Coverage 79.7%, both thresholds PASS.
Frontend: typecheck clean, lint 0 errors, 2933 tests pass.

One `comment-report` advisory finding was raised **and fixed** rather than
suppressed: `mint` asserted a containment precondition its signature did not
enforce. Fixed with the `containedPath` witness type (RR-0L9DUD), so an
unchecked path now fails to compile.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran in parallel: `cranky-code-reviewer` for quality and
`rela-security-reviewer` for security. Both independently found the `exec_id`
defect, which is the one that mattered.

Every finding was verified against the code before acting — the directory-200
and immortal-token claims were each reproduced with a throwaway probe first.

**Review Responses:** RR-AG4SG8 (significant, addressed), RR-GX7PIJ
(significant, addressed), RR-I5KBFX, RR-0L9DUD, RR-UXH1KY, RR-385HBF, RR-Z8WFJ2
(minor, addressed), RR-A98CSH (nit, addressed), RR-U463I9, RR-T7Z6UW (minor,
deferred with follow-up tickets TKT-3MDPTH / TKT-58ID95).

No critical findings. Neither deferred item is a regression: both describe
pre-existing exposure that this ticket does not widen, and each has a ticket so
it is not lost.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **PASS** — `/api/open-file` is not mounted on any build; the route walk test
covers `/api/command-file/` instead.
2. **PASS (in substance)** — no *code* matches; two explanatory comments
mention the old route by name, which the literal grep in the AC did not
anticipate.
3. **PASS** — one Download link per file item, `Content-Disposition:
attachment` asserted in `TestHandleCommandFile_ServesHardenedDownload`; frontend
pinned by `CommandModal.test.ts`.
4. **PASS** — `TestHandleCommandFile_ReauthorizesPerDownload` covers both the
revoked-permission and `--read-only` cases. Mutation-tested: removing the
re-check fails both subtests.
5. **PASS** — `TestCommandFileStore_ExpiresAfterRelease`, plus
`_UnreleasedTokensStillExpire` for the backstop added during review.
6. **PASS** — `TestMintFileToken_StripsPath` and
`TestCommandExecEmitsTokenNotPath`; `_OutsideBytesAreUnreachable` asserts
containment as a property rather than as a rejection.

**Live server verification (rela-server + the demo project, port 8799):**

- `/api/open-file` and `/api/open-url` both **404** on a running server (AC-1).
- A real `generate-pdf` run emitted `{"type":"file","token":"...","label":...}`
with **no path** (AC-6).
- Downloading that token returned a valid `%PDF-1.4` with `Content-Disposition:
attachment`, `nosniff`, sandbox CSP and `no-store` (AC-3).
- **One process, one token, three principals: alice 200, bob 404, unknown 404.**
This is the decisive AC-4 evidence — authorization is evaluated per download
against the caller, not baked in at mint time.

Two things learned on the live server, neither a defect in this change:

- `acl.yaml` is read at **startup**, not hot-reloaded, so revoking a permission
needs a restart before any endpoint sees it (a fresh command *exec* also still
passed until restart). Pre-existing; the per-download re-check is proven by the
cross-principal case above, which is in-process.
- The demo project needed `permission: command:generate-pdf` plus a matching
role grant before the command would run at all — the command ACL gate from
  #1180 working as designed.

**Still NOT done:** verification on the actual headless remote deployment. The
original symptom (`xdg-open` silently no-opping) is only observable there, and a
local macOS server cannot reproduce it.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

`GUIDE-data-entry.md` (File Downloads section, message-type table, the dead
`open` row removed, `auto_open` marked inert + migration pointer) and
`GUIDE-server-security.md` (section 5 rewritten, section 6 removed and the rest
renumbered, TOCTOU section rewritten to admit the window widened). `docs/`
regenerated and verified idempotent.

Demo project fixed too: its `generate-pdf` command wrote to `/tmp` and set
`action: "open"`, which would now produce no Download button.

**Docs Checklist:** DOCS-A13B5H (done).

**Implementation Checklist:** IMPL-1YYH8O (done). Created retroactively — the
ticket moved `ready` → `review` directly, so the `in-progress` automation that
would normally create it never fired. Caught by `rela validate`, which the Go
test suite does not cover: the gates read ticket data, not code.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!-- Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
