---
id: DOCS-BU0P1D
type: docs-checklist
title: 'Documentation: verify renderer security assumptions (Milkdown passthrough, flattenToLine)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] ~~Function/type docs if public API~~ (N/A: no public API added; the only
production edit is a godoc addition to an unexported function)

`flattenToLine`'s godoc already explained that its safety comes from the
PARSER's definition of a line ending rather than from the function itself. That
is the load-bearing sentence, and it now names its evidence: goldmark by the Go
tests in the same package, marked by
`frontend/src/utils/markdownLineEndings.test.ts`. Without the pointer, a reader
auditing the claim has to rediscover which renderers were actually checked —
which is how #1594 came to be filed in the first place.

Both new test files open with a comment stating what is being verified and why
the property is not self-evident: for Milkdown, that the `html` node renders its
value as a text child rather than through an `innerHTML` sink, and that we do
not own that behaviour; for marked, that the five surviving whitespace
characters are safe only because no parser treats them as line endings.

Each also records why its assertions are written the way they are — observable
consequence rather than DOM-spec shape, code points rather than literal
characters — so a later editor does not "simplify" the test into a tautology.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing surface changed)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; this adds
tests for existing behaviour)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI surface changed)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no CHANGELOG file — verified
— and release notes are generated from commits; the commit message carries the
why)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)

`docs/webhooks.md` already documents the flattening behaviour, which is
unchanged by this ticket. Nothing there needed editing: the finding was that an
existing, correctly-documented guarantee lacked a test on one of its two render
paths, not that the documented behaviour was wrong.
