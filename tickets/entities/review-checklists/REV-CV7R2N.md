---
id: REV-CV7R2N
type: review-checklist
title: 'Review: Operator-defined entity type conversions'
status: done
---

## Code Review

- [x] Conversion definitions are operator-owned; request payloads cannot
  override mappings.
- [x] Preview and write paths enforce the source-type conversion ACL and
  validate mapped properties and incident relations.
- [x] Rekeying reuses rename aliases and history lineage; faced writes are
  explicitly refused until face-scoped history migration is supported.
- [x] CI findings addressed: generated docs are sourced from docs-project,
  and this ticket is linked to its required feature and concept.
- [x] No unresolved review findings.

## Verification

- [x] Go tests and frontend typecheck/build passed before opening the PR.
- [x] `just docs-check` passed after committing the generated documentation.
