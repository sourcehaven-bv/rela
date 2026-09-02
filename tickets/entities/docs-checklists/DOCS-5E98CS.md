---
id: DOCS-5E98CS
type: docs-checklist
title: 'Documentation: audit rejected attachment uploads'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `auditRejectedUpload`'s godoc carries
three decisions a reader would otherwise have to reverse-engineer: **why** a
refused upload is worth recording (it may be an attempt to place a disallowed
type or malware, and the log is the only place that question survives); **why**
it reuses `OpDeniedWrite` rather than a new op (so one filter answers "what
uploads were refused?" without the operator knowing there are two kinds); and
**why** it records only `ErrRejected` (a size cap or an at-capacity property is
a client error, and auditing those would dilute the op until it distinguishes
nothing).
- [x] Function/type docs if public API — none added; the helper is unexported.

**One placement note is load-bearing.** The godoc states that the call lives at
the upload call site rather than inside `writeAttachmentWriteError`, where the
422 is written, because that helper is a package function with neither the audit
sink nor the entity in scope. Without it, moving the call "next to the response"
looks like an obvious tidy-up and silently drops the record.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: applies the existing audit rules rather than
changing them)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG; releases come from commit
history — `docs/releasing.md`)
- [x] ~~API docs updated~~ (N/A, and the reasoning is worth stating rather than
waving through: `docs/audit-log.md` documents the record *shape* and the
`triggered_by` vocabulary. This change adds an **occurrence** of an
already-documented op — `denied-write`, whose own doc already frames it as
*"what did this user try to do that they weren't allowed to?"* — not a new kind
of record, a new op, or a new field. Listing every site that emits an existing
op would make the guide a call-site index that goes stale on the next refactor,
which is precisely the duplication the comment-lint `duplication` rule exists to
discourage.)

If a future change adds a genuinely new op or field, that *does* belong in the
guide's `triggered_by` / record-shape sections.
