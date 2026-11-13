---
id: RR-3MPHI14
type: review-response
title: 'Design: run-wide generations'
finding: flow.Instance.Enter adds one to the generation; run-wide generations need the generation set explicitly.
severity: minor
resolution: 'Plan updated: the engine sets the generation after Enter (highest in the run plus one); the shared flow package is not changed.'
status: addressed
---
