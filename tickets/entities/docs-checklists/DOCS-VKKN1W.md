---
id: DOCS-VKKN1W
type: docs-checklist
title: 'Docs: field-level redaction on the appbuild gated read paths'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`buildFieldRedactor` carries the reasoning that is not inferable from the code:
why `NopRedactor` is correct for two distinct no-op cases, why a compile
failure is an **error** rather than a fallback (RR-GKCZO5), and why the
resolver is built to completion before it escapes (`WithMachines` is the one
mutator on an otherwise-immutable value).

The `Services.fieldRedactor` field documents why it is a field and not an
accessor — `Services` is at its plimsoll ceiling, and a field preserves
construct-then-publish.

Two KNOWN LIMITATION blocks were **deleted** rather than edited, since the
limitation is gone; each site now states positively what applies. The
`GatedReads` godoc names the relation carve-out explicitly (RR-9WKQ2M) so the
guarantee is not read wider than it is.

## Project Documentation

- [x] ~~README updated~~ (N/A: no new command, flag or entry point)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate~~ (N/A: no CLI surface change)

CLAUDE.md needed **no edit**, and that is the point worth recording: its
read-side rule ("Read-out paths go through visibility wrappers, base readers
stay ungated", DEC-ZBI39P) already stated the intended behaviour. This change
did not introduce a pattern — it made three paths match the pattern already
written down. Adding prose would have implied a new rule where the real defect
was code drifting from an existing one.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG.md in this repo; release
notes are generated from commits)
- [x] API docs updated (if applicable)

`docs-project/entities/guides/GUIDE-scheduled-tasks.md` stated that field
policy did **not** apply to scheduled tasks — true when written, false after
this change. Corrected at the source entity and regenerated into
`docs/scheduled-tasks.md` via `./scripts/generate-docs.sh`; editing only the
generated file fails the `Docs` CI check. The note now also says when the
behaviour changed, so an operator reading it against an older binary is not
misled in the opposite direction.
