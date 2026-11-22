---
id: RR-JUPGGPX
type: review-response
title: buildFlowEngine sets s.flowForms as a hidden side effect
finding: 'internal/appbuild/flowruntime.go: the caller assigns s.flowEngine but the forms are written inside the builder, making assembly order matter. Return the forms from the builder or keep them with the engine.'
severity: minor
resolution: 'Fixed with RR-YNSWLUM: buildFlowEngine only validates step forms; it no longer sets s.flowForms.'
status: addressed
---
