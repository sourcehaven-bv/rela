---
id: RR-3YAKUC
type: review-response
title: Served relied on a zero header id
finding: Served was Header.ID != "".
severity: nit
resolution: ResolvedHeader carries an unexported served flag set by ResolveHeaders.
status: addressed
---
