---
id: IMPL-B7YDUP
type: implementation-checklist
title: 'Implementation: ZIP-container documents (.docx, .xlsx, .odt, .epub) rejected on upload: mimeCompatible does not tolerate a sniffed application/zip'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`extensionMatchesSniff(ext, sniffed)` tolerates a sniffed `application/zip` for
the extensions in `zipContainerExtensions` (OOXML, OpenDocument incl. templates,
EPUB). Executable/installer and macro-carrying containers are rejected earlier
by `deniedExtensions`. Unit tests in `internal/attachment`, end-to-end tests
through the real upload handler in `internal/dataentry`.

Reworked twice under review. First keyed by MIME type, which routed the decision
through `mime.TypeByExtension` and the OS MIME database (RR-7C5D7T); now keyed
by extension so the verdict is decided in rela's own source. Then the tests were
rewritten to assert which mechanism rejects (RR-XNPEAE), and `mimeCompatible`
collapsed into a single-input predicate with the extension normalized once
(RR-3ECTBR).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertions compare against HTTP status codes and error-message fragments, which are the contract under test)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: inputs are filenames and raw bytes)
- [x] Property comparisons use original object, not hardcoded strings

Table-driven with `t.Run` subtests throughout. `zipBytes` builds a real archive
with `archive/zip` rather than asserting an assumed sniff result — the
assumption is what caused the bug. `TestMIME_AllowsZipContainerDocuments`
iterates `zipContainerExtensions` itself, so a new entry cannot be added without
being exercised. One shared `newAttachmentApp(t, metaYAML)` builds both HTTP
fixtures.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Before the fix, all seven container formats were rejected and `.zip` passed.
After: every extension in `zipContainerExtensions` plus `.zip` uploads and reads
back byte-identical through `PUT`/`GET /api/v1/tickets/{id}/_attachments/...`.
Uppercase (`.DOCX`) and extension-less names behave correctly.

Narrowness verified both ways. Still rejected with ZIP bytes: `.jar`, `.war`,
`.apk`, `.xpi`, `.crx`, `.appx`, `.ipa` and the macro-enabled office formats
(deny set), plus `.pdf`/`.png` (genuine polyglots).

Each test group was confirmed to FAIL against the code it guards — the original
fix (4 endpoint + 8 processor rejections), and the mechanism assertion (removing
`.jar` from `deniedExtensions` now fails with "rejected by the wrong check",
where the previous test passed).

Cross-host: the whole suite produces identical verdicts on macOS (populated MIME
database) and in `golang:1.26-alpine` with no `/etc/mime.types`. That run is
what caught `.docm`/`.xlsm` being accepted on a minimal image while rejected
locally.

Gates: `go build ./...`, `just arch-lint`, `just lint` (0 issues), `just
comment-lint`, `just coverage-check` (79.5%), `just docs` idempotent, full suite
green except a pre-existing `internal/lock` flake confirmed on unmodified HEAD.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The fix narrows rather than widens versus the obvious one-line alternative (`if
sniffed == "application/zip" { return true }`), which would have admitted
`.jar`/`.apk` and excused any claim over ZIP bytes. Extension normalization
happens once in `Process` and is threaded down, so one rule has one home.

Docs corrected in `docs-project/` (generated `docs/` follows): the guide
previously said office documents sniff as `application/octet-stream` — the same
wrong belief that caused the bug — and now states the deny groups and why they
are matched on the extension rather than the implied type.
