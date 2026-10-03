---
id: DOCS-0MW4OE
type: docs-checklist
title: 'Documentation: Autosave conflict resolution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (base-tracking rules on propBase, queuedProps, Conflicts, mergeServerResponse; token design on fieldversions.go; gated re-read on servedAfterWrite / writeLostRace)
- [x] Function/type docs if public API (v1.FieldVersions, Preconditions, FieldConflicts, Conflict, EditState; frontend types in types/entity.ts)

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-visible setup or CLI change)
- [x] CLAUDE.md updated (if new patterns): frontend/CLAUDE.md "Autosave conflicts" section with the base-tracking rules
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog; release notes come from PRs)
- [x] API docs updated (if applicable): docs/data-entry/api-reference.md "Per-field preconditions (`_versions`, `preconditions`)", including the unguarded incoming lists and the view entry without a relations token
