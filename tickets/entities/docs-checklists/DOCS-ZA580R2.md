---
id: DOCS-ZA580R2
type: docs-checklist
title: 'Docs: Store content hash at write time and add a fingerprint column'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

Migration 0042 and sqlite `elementDDL` state what the columns hold, why the
address is an index expression, the clearing trigger, the drain cost, the
encoding and overflow limits, and that a change to the canonical hash needs a
clearing migration. `element.go` in each store documents the write, fill and
heal paths, the SKIP LOCKED rule and why an element failure after a committed
write is logged.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing feature; step 2 of FEAT-CMVZQFI adds storage columns only)
- [x] CLAUDE.md updated (if new patterns) (`.claude/rules/storage.md` gains "Every live row carries its reconciliation element", the rule for any new write path on a database backend)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; the PR body notes migration 0042 and sqlite v34)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API changes)
