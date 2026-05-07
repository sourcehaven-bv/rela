---
id: RR-P7XKC
type: review-response
title: RelationCards reconciliation effort estimate (0.5d) is too low given the diff-accumulator refactor required
finding: |-
    Plan estimates 0.5d for RelationCards reconciliation. Per the critical finding on RelationCards, the right fix is either a per-row autoSave prop with self-resetting diff, or a flush+reset method exposed for the form to drive. Both involve real test surface (relation-cards has its own e2e). 0.5d is low.

    Re-estimate to 1d minimum. Add to scope: per-row immediate persistence in auto-save mode; tests that auto-save on a card-relation form persists each add/remove/property-edit independently and the cumulative-diff state machine doesn't double-fire any of them.
severity: minor
resolution: RelationCards work fully split out to TKT-B9SXH. Effort for this ticket re-estimated to L (~9d) with explicit breakdown including 1.5d Puppeteer-driven manual QA phase.
status: addressed
---
