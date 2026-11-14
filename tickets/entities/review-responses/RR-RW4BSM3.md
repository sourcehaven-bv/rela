---
id: RR-RW4BSM3
type: review-response
title: Bare entity.Ref in t.flows.run breaks TestNoNewBareRefs
finding: 'flows.go read the _flow-run header with entity.Ref{ID: id}, a bare ref that internal/archguard TestNoNewBareRefs rejects, so CI fails.'
severity: critical
resolution: The read names entity.ImplicitFace, since _flow-run is faceless; the archguard test passes.
status: addressed
---
