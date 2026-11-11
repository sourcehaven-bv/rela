---
id: REV-C4GJ4VY
type: review-checklist
title: 'Review: Attachments in channel messages'
started: "2026-11-10"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Also clean: `just arch-lint`, `just plimsoll`, the channels e2e spec (18 tests),
and the pgchannels conformance suite on postgres, including the Files* cases.

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 30 linked, none open.

- Design review (15): RR-1S59FXJ, RR-3OMK1OG, RR-EHP4W6F, RR-V5EDSSH,
RR-ZYNU642, RR-89ZZ36N, RR-5DUDWNZ, RR-B3BW79V, RR-DI9T66A, RR-DQWP3T4,
RR-K09BZ2V, RR-NOCSW3T, RR-NXYFUR5, RR-XCTD6X9, RR-YLUPO3O. All addressed.
- Code review (15): RR-3ULUOWI, RR-VE5OZPE, RR-MVX6N4J, RR-SHRJDT9,
RR-K8MS13D, RR-XND05MV, RR-1ZPGGH3, RR-AXVWFPY, RR-6Y373SA, RR-XDYSD18,
RR-4FMHHR3, RR-2Z6JGM4, RR-MHZEVYA, RR-KQ1958O addressed; RR-M82A6QZ (minor)
wont-fix with its reason recorded.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. AC1 PASS: `realtimecfg` load tests accept `max`/`max_size`/`accept` and
refuse `list`, `to`, an oversized `max_size`, `file` on an ephemeral type, and a
required file on a trigger or reminder post, naming the property.
2. AC2 PASS: dataentry multipart post and download test; manual HTTP run
showed identical bytes with `attachment`, `nosniff` and `no-store` headers.
3. AC3 PASS: dataentry download table (wrong id, bad id, wrong seq, other
channel, hidden anchor) all 404; a removed message's file is 404.
4. AC4 PASS: realtime and dataentry refusal tests (accept, size, count)
return 422/413 and store nothing; `channelstest` FilesCaps on every backend.
5. AC5 PASS: realtime transform and reject tests with a fake runner.
6. AC6 PASS: `channelstest` file cases (append, OpenFile, Remove, Delete,
Tombstone, Purge, Rekey) pass on fs, sqlite and postgres; a JSON value for a
`file` property is refused.
7. AC7 PASS: MCP `post_message` with files and `read_message_file` tests;
CLI `channel post --file` and `channel file --to` tests and a manual run.
8. AC8 PASS: manual run found the `denied-write` audit record without
content; the attachment audit test pins its shape.
9. AC9 PASS: Playwright "message files" tests post a PNG, show the chip and
the preview to a second reader, and download identical bytes.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HVPLAAL (realtime, CLI reference and MCP guides).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done, see TKT-UFV01M)

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
