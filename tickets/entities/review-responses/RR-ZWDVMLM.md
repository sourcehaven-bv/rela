---
id: RR-ZWDVMLM
type: review-response
title: A when without a compiled condition bound as unconditional
finding: grantConds treated a relation missing from the bound map as unconditional; so a policy mistake would fail open.
severity: minor
resolution: 'bindGrantConds binds a when: without a compiled condition to ''no edge counts''. TestBindGrantConds_UncompiledWhenFailsClosed.'
status: addressed
---
