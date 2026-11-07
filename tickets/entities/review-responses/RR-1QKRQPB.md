---
id: RR-1QKRQPB
type: review-response
title: _flow-run must not be its own _carries target
finding: If the target list is computed after the type is added, _flow-run is a faceless target of itself and readQuery recurses without end.
severity: critical
resolution: 'Plan changed: targets are computed from operator types before the managed types are added; readQuery skips a reserved subject type; a test pins that no target is reserved.'
status: addressed
---
