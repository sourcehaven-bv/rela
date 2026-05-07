---
id: RR-Y8T8A
type: review-response
title: Per-edge identity (from, type, to) — graph doesn't enforce uniqueness, plan never confirms it
finding: |-
    Plan classifies diff by matching (target_id, type). graph.AddEdge (graph.go:166-173) appends without uniqueness check; GetEdge returns first match; RemoveEdge removes ALL matches. So the graph data structure does NOT enforce uniqueness on (from, type, to). In practice the file layout enforces it (one file per tuple, repo.WriteRelation rewrites), but parallel edges are representable.

    Fix: add to Decisions: 'We assume (from, type, to) uniquely identifies an edge. Enforced by file layout (one file per tuple, repo.WriteRelation overwrites). If in-memory graph has duplicates due to corruption or loader bug, the diff treats them as a single logical edge: update-meta writes once, remove deletes the file (RemoveEdge then mirrors by removing all matching edges). Validation: tx.OutgoingEdges iterated; on duplicates, last write wins for the diff-key map.' Add unit test: graph with two duplicate edges, PATCH that touches that relation type, asserts disk converges to one canonical file.
severity: minor
resolution: 'Codebase facts section: ''graph allows duplicate edges on (from, type, to). File layout enforces uniqueness; we document the assumption.'' Diff classifier treats duplicates as a single logical edge.'
status: addressed
---
