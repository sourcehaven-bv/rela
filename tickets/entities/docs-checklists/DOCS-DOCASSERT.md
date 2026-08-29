---
id: DOCS-DOCASSERT
type: docs-checklist
title: 'Documentation: Executable manuals — assertions in the rela-docs doc language'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The godoc on each verb states WHY rather than what — why authorization goes
through `acl.Declarative` instead of re-reading `policy.Roles`, why `instance`
is excluded from `identical_to`, why an unknown `who`/`type`/`as` is refused.
Those are the decisions a future reader would otherwise reverse.

## Project Documentation

- [x] README updated (if applicable) — no README change: the doc language is
      documented in its own guide, which README links.
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new architectural
      pattern — `APIClient` follows the existing consumer-side-interface and
      build-tagged-seam rules CLAUDE.md already states)
- [x] Help text accurate (if CLI changes) — no new flags; the verbs are doc
      language, not CLI surface.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no CHANGELOG.md)
- [x] API docs updated (if applicable) — `docs/rela-docs.md` gained an
      "Assertions" section (resolver table rows, `shows{}`/`refuses{}`/
      `permits{}`/`api{}`, the `unassigned=` argument, and the `identical_to`
      scope note). Edited at the SOURCE (`docs-project/entities/guides/`) and
      regenerated, since `docs/rela-docs.md` is generated and hand edits are
      overwritten by `just docs`.

**Proof rather than prose:** the example handbook
(`prototypes/data-entry/manual/tickets-manual.md`) now asserts its own
access-control claims, so the documentation of this feature is itself an
instance of it.
