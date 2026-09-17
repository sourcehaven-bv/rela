---
id: RR-FBI16Z
type: review-response
title: Default value on the cache-key mode parameter undermined its own guard
finding: 'listCacheKey and fetchListInternal both declared mode: ''page'' | ''all'' = ''page''. The comment above listCacheKey argues a future mode must not silently opt out of invalidation, but the default lets a future caller silently opt INTO the wrong cache slot -- the same class of defect from the other direction.'
severity: significant
resolution: Made mode required on both functions and moved the justification into a comment at fetchListInternal. The two call sites are three lines apart, so naming the mode explicitly costs nothing.
status: addressed
---
