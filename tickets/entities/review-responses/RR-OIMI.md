---
id: RR-OIMI
type: review-response
title: Multi-snapshot read in analyzeRelationOrder
finding: |-
    analyze.go:478 captures s := a.State(). findOrderIssues (called from it) reaches a.State() again at line 562 for DisplayTitle. CLAUDE.md rule: "Capture state once per operation. ... multiple loads against the underlying atomic.Pointer can observe different snapshots if a reload lands between them."

    Fix: pass s.Meta (or *AppState) into findOrderIssues and use it consistently.
severity: significant
resolution: findOrderIssues now takes the metamodel snapshot as an explicit argument (passed in by analyzeRelationOrder from its single s := a.State() capture). No more a.State() reload during the analyze run. Made findOrderIssues a package function rather than method to make the dependency explicit.
status: addressed
---
