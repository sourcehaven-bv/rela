---
id: DOCS-23G0PV
type: docs-checklist
title: 'Documentation: analyze relation-files'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the `CheckRelationFilenames` godoc
explains the *mechanism* (fsstore keys on filename and never opens the file) and
the *observed symptom* (a cardinality error against an innocent entity), because
neither is reconstructable from the code and the symptom is what a maintainer
will actually arrive with.
- [x] Function/type docs if public API — `RelationFilenameReason`'s three
constants each document the **consequence**, not just the condition: skipped
entirely / indexed as the wrong triple / loads with an empty type. That is what
tells an operator how urgent each finding is.

**Two comments were rewritten because they asserted something false**, which is
worth recording since both survived my own review and were caught by the code
reviewer:

1. The legacy-`type:` comment claimed those files "work, because the store keys
on the FILENAME". False — `mdCodec` reads content, so they load with an empty
relation type (RR-YB7XX8).
2. The `frontmatterScalars` godoc justified a hand-rolled parser by an arch-lint
rule that did not apply — `yaml` is a `commonVendor` and `internal/frontmatter`
exists for exactly this (RR-DDZ02R). The scanner is gone.

**One comment now carries an invariant it did not before:**
`splitRelationFilename`'s doc states it MUST stay identical to
`fsstore.parseRelationFilename`, names why the duplication is correct (arch-lint
forbids the dependency; exporting a storage detail would be worse), and names
the test that enforces it. The fsstore side names this one back.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern. The check follows the existing
`CheckRelationOrder` / `FindOrphanedTempFiles` shapes.)
- [x] Help text accurate — the kong `help:` string for `analyze relation-files`
states what it finds in one line.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG; releases come from commit
history — `docs/releasing.md`)
- [x] API docs updated — new `rela analyze relation-files` section in
`docs-project/entities/guides/GUIDE-cli-reference.md`, regenerated into
`docs/cli-reference.md` (generated file, never edited directly; regeneration
touched only it).

The section leads with **why the symptom appears somewhere else** — a
cardinality error against an innocent entity — because that is the state an
operator is in when they go looking. It then tables the three findings with
their consequences, and gives the fix for each: rename the file, or rename the
key.

It also states that renaming an entity through rela keeps filename and content
in step automatically, so nobody reads the check as evidence that rename is
broken. It is not — `fsstore.renameEntity` is correct, and this check is for
files moved, hand-edited or merged outside rela.
