---
id: DOCS-UGZLQ8
type: docs-checklist
title: 'Docs: Cardinality analysis: batch relation counts and count visible edges only'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `schema.CardinalityReader` states that the reader is the gate and why the read is strict; `CheckCardinalityFindings` states its read cost
- [x] Function/type docs if public API: `CheckCardinality`, `CheckCardinalityFindings`, `CardinalityFinding`, `Ungated` and `Reader.FilterRelationsStrict` documented; stale "raw counts" comments in dataentry, MCP and appbuild rewritten

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing command or config change)
- [x] ~~CLAUDE.md updated~~ (N/A: follows the existing gate-before-fold rule; no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI flag or output change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the project keeps no changelog file; the PR title records the fix)
- [x] API docs updated: `docs/acl-security.md` (via GUIDE-acl-security) no longer lists cardinality counts as a residual channel
