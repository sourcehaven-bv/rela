---
id: REV-TM3B0I
type: review-checklist
title: 'Review: ZIP-container documents (.docx, .xlsx, .odt, .epub) rejected on upload: mimeCompatible does not tolerate a sniffed application/zip'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full suite green except `internal/lock`'s
`TestMemoryLocker_AbandonedAcquireReleases`, which fails ~1 in 20 on an
unmodified `HEAD` worktree too. Pre-existing flake in a package this change does
not touch; reported to the user rather than fixed here.

`just lint` 0 issues; `comment-lint` gate clean (14177 comments);
`coverage-check` 79.5% total with all package floors satisfied; `arch-lint`
clean; `just docs` regeneration idempotent. Additionally the attachment and
dataentry suites were run in `golang:1.26-alpine` with no `/etc/mime.types` and
produce identical verdicts.

No new advisory comment findings introduced; the OS-MIME rationale is stated
once on `zipContainerExtensions` and cited from the tests rather than restated.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviews ran: `rela-security-reviewer` (this change touches the upload
security boundary) and `cranky-code-reviewer`. No critical findings from either.
Both significant findings are fixed, and each was verified by reproducing the
defect first.

**Review Responses:** RR-7C5D7T (significant, addressed), RR-XNPEAE
(significant, addressed), RR-SPM9GS (minor, addressed), RR-1QO0BP (minor,
addressed), RR-3ECTBR (minor, addressed), RR-WM1FU5 (nit, addressed), RR-BH2JHD
(nit, addressed).

The two that changed the design:

- RR-7C5D7T — the first fix keyed its allowlist by MIME type, routing a security
decision through `mime.TypeByExtension` and the OS MIME database. Go's builtin
table carries only 6 of the relevant extensions, so on a minimal image the fix
stopped applying to OpenDocument/EPUB while `.jar`/`.xpi` became accepted. Now
keyed by extension, with executables and macro formats in `deniedExtensions`.
- RR-XNPEAE — the tests asserted only `ErrRejected`, which cannot distinguish the
host-independent deny set from the host-dependent claim check. Removing `.jar`
from the deny set left them passing. They now assert which mechanism fired.

Diff contains no unrelated changes: five files, all attachment MIME validation,
its tests, and the corresponding doc source plus its generated mirror.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS — every ZIP-container document format uploads and reads back
byte-identical through the real HTTP handler
(`TestAttachmentUpload_ZipContainerDocuments`).
- PASS — `.zip` still uploads; PDFs and images unaffected.
- PASS — the tolerance stays narrow: genuine polyglots and executable/macro
containers are still rejected, each by the intended mechanism
(`TestMIME_ZipRejectionsNameTheirMechanism`).
- PASS — verdicts are identical with and without an OS MIME database.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a bug fix)

The guide was carrying the same wrong belief that caused the bug ("office
documents sniff this way" of `application/octet-stream`), so correcting it is
part of the fix rather than an enhancement.
`docs-project/entities/guides/GUIDE-attachment-security.md` now documents the
ZIP-container behaviour, both deny groups, and why they are matched on the
extension; `docs/attachment-security.md` regenerated.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

Pending: the work is committed-ready but no PR has been opened, because the user
has not asked for one. Run `/pr` when they do.
