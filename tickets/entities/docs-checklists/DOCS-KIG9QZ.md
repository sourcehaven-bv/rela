---
id: DOCS-KIG9QZ
type: docs-checklist
title: 'Docs: ACL test coverage for per-recipient scheduled mail'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

No production code changed, so no godoc. All the documentation here is in the
test file, and for a test-only ticket that is the deliverable rather than a
side-effect: a redaction test whose reasoning is not written down is one
refactor away from silently testing nothing.

The file header states what was missing and why it mattered — the only test
touching `RunScheduledTemplate` installed `NopRedactor` and no `Declarative`, so
it exercised the rendering plumbing with access control switched off, and the
regression it would have caught (one recipient receiving another's data) is
silent to everyone except the wrong recipient.

Three decisions carry comments at the point someone would undo them:

- **Why two tests, not one.** A denied row hides its fields for free, so a
single test cannot distinguish "the field was redacted" from "the whole row was
gone". The mutation results prove that split was load-bearing — disabling only
field redaction reddens only the field test.
- **Why each test has a positive control.** Absence assertions pass trivially
against a renderer that emits nothing, and "the redaction test quietly stopped
rendering" is the standard way this class of test dies unnoticed. So the wider
role's message is asserted to CONTAIN the row and the field before the narrower
role's is asserted not to.
- **Why the assertion is on the rendered text.** The claim is about what the
recipient receives; asserting on the model one layer up would leave a renderer
that reintroduced a redacted value uncaught.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern — it applies
the existing `visibilitytest` recipe for building a real policy stack)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: no behaviour changed)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary or wiring change)

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLI reference updated~~ (N/A: no command or flag changes)
- [x] ~~API docs updated~~ (N/A: no HTTP surface change)

## Rationale for N/A

Nothing observable changed. The feature behaved correctly before this ticket and
behaves identically after; what changed is that a regression in per-recipient
ACL scoping now fails CI instead of shipping.

Deliberately NOT documented in `docs/`: the ACL guide already documents row
denial and `visible:` field redaction as general mechanisms, and scheduled mail
inherits them rather than defining its own rules. Adding "and this also applies
to scheduled mail" would start a list that has to be maintained everywhere the
mechanisms apply, and would go stale the first time a surface was added without
updating it. The invariant is better expressed by the test than by prose.

Worth recording for whoever revisits this: the gap existed because PLAN-XMWT23
AC7 promised exactly this test ("Deny one row and redact one field... assert
both are absent") and the promise was not kept in the diff. The checklist said
the work was done. That is a process observation rather than a documentation
one, but it is the reason the issue had to come from an external review instead
of from CI.
