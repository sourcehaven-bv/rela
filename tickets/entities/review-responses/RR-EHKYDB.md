---
id: RR-EHKYDB
type: review-response
title: 'Bidirectional edges: tolerating ErrConflict destroys one edge and corrupts the other'
finding: 'The plan copied rename_relation_type''s ErrConflict tolerance (steps.go:542), whose comment reads already created by a prior crashed run - fall through to delete. That inference is sound there because the destination triple uses a type name nothing else writes. For a reversal the destination triple is in the SAME namespace being read, so ErrConflict is ambiguous between a crashed prior run and a legitimate distinct edge. Verified empirically on a memstore: with A--blocks-->B and B--blocks-->A both stored, the proposed sequence leaves ONE edge, and the survivor is A-->B carrying w=BA - the other edge''s properties. That is silent corruption, not merely loss. On fs and memory backends there is no version capture at all (newCapturer returns nil when Versions is nil, run.go:307), so the original is unrecoverable.'
severity: critical
status: open
---
