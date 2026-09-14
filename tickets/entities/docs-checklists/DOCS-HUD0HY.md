---
id: DOCS-HUD0HY
type: docs-checklist
title: 'Documentation: flatten interpolated values rather than the operator''s template'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`flattenToLine`'s godoc was rewritten to say what it now guards: it collapses
newlines and NULs in ONE INTERPOLATED VALUE, and the threat is the payload, not
the template. It records why the flattening sits in `interpolate` rather than on
the finished string — the finished string cannot tell the two sources apart —
and why it never returns an error. The `append_section` case in `applySteps`
carries a matching note that the newlines reaching it are the operator's.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

None applicable: no CLI surface changed, and this is a behaviour change within
an existing documented feature rather than a new pattern. The user-facing
documentation is `docs/webhooks.md`, covered below.

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

`docs/webhooks.md` gains "Multi-line content is yours; multi-line values are
not", which shows a structured `content:` block and states the rule plainly:
every interpolated value is flattened to one line first, and the shape written
in `content:` is not touched. It names the consequence that motivates the
asymmetry — a producer that could emit a newline could emit `## Heading`, which
would become a sibling of the section it was appended to.

`docs/webhooks.md` is one of five HANDWRITTEN files under `docs/` (with
customisation, idp-webhook-provisioning, releasing, transforms). It is edited
directly and correctly so: the generator `scripts/generate-docs.lua` writes only
files that have a backing guide entity in `docs-project/entities/`, and stamps
each with an "auto-generated ... Do not edit directly" header.
`docs/webhooks.md` has no such header and no backing entity. Verified by running
`just docs` and confirming the file's checksum was unchanged and `git status`
stayed clean, so this prose will not be reverted by a later docs run.

The repo has no CHANGELOG file; release notes are generated from commits, and
the commit message carries the why.
