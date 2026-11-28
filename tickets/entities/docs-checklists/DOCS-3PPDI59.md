---
id: DOCS-3PPDI59
type: docs-checklist
title: 'Docs: roll-ups along a chain and over siblings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: gate-before-fold order, earliest-arrival dedupe, per-source cap and HopCap, guarded `_order_in` cast, keyset `pageAfter` reset in the pg leaf
- [x] Function/type docs if public API: godoc on `store.AlongWalk`, `SiblingWalk`, `RollupResult.OverCap`, `rollup.Set.ReadTypes`, `metamodel.ParseAlongSegment`; comment-lint and comment-body gates clean

## Project Documentation

- [x] README updated (if applicable): `examples/omnifocus/README.md` rewritten for the roll-ups
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the collection-reads and storage rules as written)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-metamodel.md`: `along:`, `siblings_before`, nesting, roll-ups in list conditions, absent values, refresh scope; regenerated
- [x] `GUIDE-acl-security.md`: gate before fold, per-source walk bounds and what they disclose; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the list config with `refresh_types` is not in the OpenAPI spec)
