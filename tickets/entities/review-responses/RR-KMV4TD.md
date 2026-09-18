---
id: RR-KMV4TD
type: review-response
title: viewQueryScope returned unscoped when cfg or meta was nil
finding: 'The nil guard folded three conditions into one: `if fn == nil || cfg == nil || meta == nil { return unscoped }`. The doc justified only the first — no resolver wired means no scopes can be declared, so unscoped is truthful. That argument does not extend to the other two: a nil metamodel is not ''no scopes are declared'', it is ''I cannot tell whether any are'', and answering it by reading everything is the wrong failure direction for a function whose whole contract is fail-closed.'
severity: significant
resolution: Split the conditions. A nil resolver func still yields an unscoped read, with the reasoning kept at the call site; a nil cfg or meta now returns an error, which every caller maps to a 500. Comment states why the two are different cases rather than leaving them to read as one.
status: addressed
---

Not reachable today — `App.Meta()` reads a published snapshot that is never
partial — so this was hardening rather than a live defect. It is recorded
because the folding is exactly how the original fail-open bugs in this area
read: one guard covering several cases whose justification only fits some of
them.
