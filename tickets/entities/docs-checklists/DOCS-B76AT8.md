---
id: DOCS-B76AT8
type: docs-checklist
title: 'Docs: Ticket gate rejects code-only and non-bug-entity PRs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the widened glob list carries a
      comment explaining why work-item entities count and why reference
      material (concepts, decisions, ideas) deliberately does not, so the
      next person does not "tidy" the exclusion away
- [x] ~~Function/type docs if public API~~ (N/A: shell step in a workflow
      file, no Go API surface)

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not document CI gate internals)
- [x] ~~CLAUDE.md updated~~ (N/A: no new code pattern — this is a policy
      widening of an existing gate, not a convention contributors write
      code against)
- [x] ~~Help text accurate~~ (N/A: no CLI change). The equivalent
      contributor-facing text is the step's own error output, which now
      documents the `Tickets-PR:` trailer alongside the `rela create`
      suggestions.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: internal CI policy, not a shipped
      user-visible change)
- [x] ~~API docs updated~~ (N/A: no API change)
