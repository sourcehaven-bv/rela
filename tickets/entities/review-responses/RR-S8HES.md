---
id: RR-S8HES
type: review-response
title: currentByTarget silently drops corrupt graph state (last-writer-wins on duplicate edges)
finding: |-
    api_v1.go:602-608 builds `currentByTarget := map[string]*model.Relation{}` then iterates outgoing edges. If duplicates exist (the very bug graph.AddEdge was just patched to fix could leave residue from imports/loaders), only the last one is captured. The diff then emits a partial removal and on commit only one file is deleted. Corrupt state persists silently.

    Fix: warn on duplicate detection:
    ```go
    if _, dup := currentByTarget[edge.To]; dup {
        slog.Warn("duplicate edge in graph", "from", entityID, "type", relType, "to", edge.To)
    }
    currentByTarget[edge.To] = edge
    ```
severity: significant
resolution: computeDiff's currentByTarget loop now logs slog.Warn on duplicate (from, type, to) detection before keeping the last edge. Surfaces graph corruption without breaking the diff.
status: addressed
---
