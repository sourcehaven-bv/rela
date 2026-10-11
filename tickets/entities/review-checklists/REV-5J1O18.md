---
id: REV-5J1O18
type: review-checklist
title: 'Review: OpenAPI spec complete enough to drive rela-server from restish (attachments + auth)'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): exit 0
- [x] Lint clean (`just lint`): 0 issues; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`): no unresolvable doc links
- [x] Coverage maintained (`just coverage-check`): PASS, total 82.9%

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-TMI6E5, RR-9NTK09)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TMI6E5, RR-9NTK09 (significant, addressed); RR-8CD5SB,
RR-XLWOKO, RR-0MILQR, RR-80EZAC, RR-FBUQP1, RR-R8C2JB (minor, addressed);
RR-97E3C3 (minor, deferred); RR-0PLJHN, RR-JT3B0X, RR-EF4ZSQ (nit, addressed);
RR-A5LJ7K (nit, wont-fix); RR-T3FF5M (nit, deferred)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-YCKA30)

**Acceptance Status:**

1. Discovery and per-type commands: PASS, amended. restish probes the root of the base URL, and the base URL must be the origin, so the spec URL is configured explicitly (docs/restish.md). `restish api sync` succeeds and lists put/get/delete-task-attachment.
2. OAuth login through Pratique and entity GET: PASS (local stack; TestSecurityScheme, TestOpenAPI_SpecNamesTheJWTHeader).
3. 400 kB PNG upload byte-identical: PASS (401,232 bytes, `cmp` identical, on the final build).
4. Same ACL, MIME filter, size cap and audit as multipart: PASS (TestAttachmentUpload_RawBodyRefusals, _RawBodyGatedAndAudited, _RejectionAuditsNormalizedName).
5. Valid OpenAPI 3.1 and every operation routable: PASS. libopenapi-validator reports valid (35 paths), run from a scratch module; TestSpecIsStructurallySound and TestOpenAPI_EveryOperationReachesAHandler in the repo.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-P3DYL4

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the user asked to explore restish locally; a PR is opened on request)

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
