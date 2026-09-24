---
id: RR-ZIB0M1
type: review-response
title: Unprimed ACL fallback repeats queries per verdict call
finding: FieldVerdicts; RelationVerdicts and TransitionVerdicts each build a bindingContext; so one GET queries the same spec three times.
severity: minor
resolution: The per-operation memo keyed (type; id; spec) serves all verdict calls on the ctx.
status: addressed
---
