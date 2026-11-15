---
id: RR-5DST37L
type: review-response
title: Two header page conformance cases skip the fallback path
finding: InvalidCursorFails and MutatingHeaderPropsDoesNotAffectStore called only the native header page.
severity: nit
resolution: Both cases now run native and pagerOnly fallback reads.
status: addressed
---
