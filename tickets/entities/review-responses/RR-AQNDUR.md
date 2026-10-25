---
id: RR-AQNDUR
type: review-response
title: filechannels due scan cost is understated, and AC 9 does not pin the dominant cost
finding: 'ListSessionsDue reuses visitAll(typ), which readDirs the root, every entity directory and every face directory, and only then filters by type (internal/channels/filechannels/file.go:803-821, 739-757, 720-760). The cost per call is O(entity dirs with channels + faces) directory listings plus the meta.yaml reads, not ''one meta.yaml read per channel of the type'' (plan line 479). The round calls it once per timer type, every round, and at 1s intervals while More is set. AC 9 counts only meta.yaml, log and audit reads. Plan change: make it ListSessionsDue(ctx, types []string, until, limit) so a round walks the tree once (on SQL use type IN (...); the index still serves each type), have AC 9 count ReadDir calls too and pin one walk per round, and state the real cost in the godoc and the risk section.'
severity: significant
resolution: 'Plan updated: ListSessionsDue takes types []string (type IN on SQL, one visitTypes walk on fs); godoc and risks state the real walk cost; AC 9 counts ReadDir and pins one walk per round.'
status: addressed
---
