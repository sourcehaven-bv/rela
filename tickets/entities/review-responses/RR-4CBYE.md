---
id: RR-4CBYE
type: review-response
title: Plan claims WithTx needs WriteRelation/DeleteRelation methods — they already exist
finding: |-
    Plan's 'Codebase gap' section says: 'writeRelationCore is not transaction-aware. Add tx.WriteRelation that stages instead.' Verified false: tx.go:103 has `func (tx *Tx) WriteRelation(rel *model.Relation) error`, tx.go:123 has `func (tx *Tx) DeleteRelation(from, relType, to string) error`. The Tx struct already accumulates addEdges/removeEdges and applyGraphMutations orders removes-before-adds correctly. Plan budgets 1 day for nonexistent work.

    Fix: rewrite 'Codebase gap' to reflect reality. The real workspace-side gap is that Workspace.UpdateRelation merges meta (workspace.go:1531) which doesn't match replacement semantics — see RR-2.
severity: critical
resolution: 'Plan corrected: tx.go:103 already has WriteRelation, tx.go:123 has DeleteRelation, Tx struct accumulates addEdges/removeEdges with correct ordering. Removed 1d budget for nonexistent work. Codebase facts section in research now reflects reality.'
status: addressed
---
