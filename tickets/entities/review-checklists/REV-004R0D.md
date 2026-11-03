---
id: REV-004R0D
type: review-checklist
title: 'Review: Release: CycloneDX SBOM per archive'
started: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go code changed; PR CI runs them)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go code changed; PR CI runs it)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go code changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4KUPKV, RR-2WH8F9 (significant, addressed);
RR-MVMV22 (minor, wont-fix); RR-4OBHUB (minor, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. SBOM per archive named `<archive>.cdx.json`: PASS (dry run on v26.10.14, 18 files).
2. Go modules and stdlib with versions and purls; job fails without stdlib: PASS (local build and v26.10.14; guard returns non-zero on a stdlib-less SBOM).
3. syft pinned by version and tarball sha256: PASS (install block passes `sha256sum -c`).
4. Non-publishing jobs run with `contents: read`: PASS (workflow default `read`; only `release`, `sbom`, `desktop` get `write`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors. This is a `chore`-kind
ticket; `docs/releasing.md` names the SBOM assets.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
