---
id: RR-VCCHQYE
type: review-response
title: PrepareUpdate does store reads on every watched update
finding: Every update of a flow subject type listed runs, read roles and bound system:flow, even when nothing could fire.
severity: significant
resolution: MayStart checks predicate starts by changed fields; machine writes skip the run list; roles are read only with open runs; forked runs settle only when a branch arrived (joinWaits).
status: addressed
---
