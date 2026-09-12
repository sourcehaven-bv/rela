---
id: DOCS-KM8O68
type: docs-checklist
title: 'Docs: Add native relation-cardinality support to validation rules (relations: block on ValidationRule)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`RelationConstraint` (min/max/where) and the `Relations` field on
`ValidationRule` carry godoc stating what the constraint asserts, not just its
shape. Two decisions are documented where someone would otherwise undo them:

- **`validValidationRuleKeys`** records *why* the allowlist exists — an unknown
key inside a validation rule was previously dropped by `yaml.Unmarshal`, which
is how the `relations:` gates went unevaluated (TKT-IFHO2L). The comment names
`TestValidValidationRuleKeysMatchStruct` as the thing that keeps it in sync with
the struct tags, so adding a field without a whitelist entry fails loudly.
- **`lua.ReadDeps.OutgoingRelations`** documents that it reads through
`VisibleReader`, so the relations counted are the ones the acting identity may
see, and that a nil reader returns nil rather than falling back to a raw handle
(RR-X9NVHI). It also states why the helper lives on `ReadDeps` at all: it lets
`internal/validation` enumerate relations without importing `internal/store`,
which `arch-lint` forbids.

`checkRelationConstraint` documents the degrade-to-no-op contract for filter and
read errors, and why that direction was chosen (a spurious violation is worse
than a skipped check, matching how the when/then paths already swallow filter
errors).

## Project Documentation

- [x] ~~README updated~~ (N/A: no new command, flag or user-visible entry point;
this extends the metamodel schema, which the README does not enumerate)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: introduces no new
architectural rule. The consumer-side `ReadDeps` helper is an application of the
existing "interfaces at the call site" / capability-bundle rules already
documented there, not a new one)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI surface change —
`rela validate` gains no flag; the same command evaluates more rules because the
schema can now express them)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG file)
- [x] API docs updated (if applicable)

`docs/metamodel.md` gains a "Relation Cardinality Validation" section: what the
block asserts, a worked `min` example (done ticket needs a completed review
checklist) and a worked `max: 0` example (no open critical review-responses), a
field table for `where`/`min`/`max`, and a note that unknown keys inside a
validation rule are now rejected at load time. The same content was added to the
source entity `docs-project/entities/guides/GUIDE-metamodel.md`, so the
generated docs and the entity graph do not drift.

## Rationale

This is a metamodel-schema feature, so the metamodel reference is the one place
that genuinely needed updating. The 14 migrated gates in `tickets/schema.yaml`
double as live worked examples of the block in use.
