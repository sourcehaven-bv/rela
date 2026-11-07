---
id: DOCS-FYXOOYB
type: docs-checklist
title: 'Docs: Reconciliation element and reader view hash'
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The `reconcile` package doc states the element, the order rule, the three
binding rules for later steps (gate before folding, stored hash only per row
without hidden fields, relation endpoints readable) and the fingerprint threat
model. `canonical.Digest` documents the view-hash role; `viewETag` and
`entityView` in dataentry document the raw-body fold and the redaction union.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing feature yet; step 1 of FEAT-CMVZQFI adds no command or endpoint)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; the rules live in the reconcile package doc where steps 2 to 5 read them)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; the PR body notes that ETag values change once)
- [x] ~~API docs updated (if applicable)~~ (N/A: ETags stay opaque strings; the OpenAPI description is unchanged)
