---
id: DOCS-GDJHQB
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the `BulkMigrator` doc states why a
      rewrite belongs in the store rather than above it (endpoints are an
      address, so a loop must be create-then-delete, which destroys a self-edge,
      merges a both-directions pair and forks version lineage); `checkSwappable`
      states why the precondition is asserted rather than assumed
- [x] Function/type docs if public API — `BulkMigrator`,
      `SwapRelationEndpoints`, `CheckSwapRelationEndpoints`, `EffectiveBound`

## Project Documentation

- [x] README updated (if applicable) — regenerated via `just docs`
- [x] ~~CLAUDE.md updated~~ (N/A: the capability follows the existing
      HeaderReader/Formatter pattern; no new convention)
- [x] ~~Help text accurate~~ (N/A: no new CLI command; the step is used through
      the existing `rela migrate gen` / `rela migrate data`)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no changelog file)
- [x] API docs updated (if applicable) — the data-migration guide gains a
      "Reversing a relation" section covering the step, the three refusals with
      their reasons, the cardinality warning and the version-history behaviour;
      the step table gains a row. `docs/data-migration.md` regenerated from it.
