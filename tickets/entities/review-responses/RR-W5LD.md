---
id: RR-W5LD
type: review-response
title: Router is net/http ServeMux, not chi — middleware composition is different
finding: 'The plan assumed chi (with route groups and Use()). Reality (router.go): plain stdlib `http.NewServeMux` with manual `mux.Handle(...)`, plus a single `reloadLockMiddleware` that wraps the whole `/api/` namespace. There are no route groups. The plan needs to either (a) wrap the entire mux in security middleware (simpler, fail-closed) or (b) introduce a thin per-route helper. Without acknowledging this, implementation will hit friction and might end up applying middleware inconsistently.'
severity: significant
resolution: 'Plan updated: security middleware is composed at the top level in router.go by wrapping the returned http.Handler. Order from outside in: requireLocalHost → requireSameOrigin → reloadLockMiddleware → mux. Streaming-route exemptions are handled by the requireSameOrigin middleware checking the request path against an exempt list (the streaming endpoints still need Host + Origin checks, just not the mutation-method gate).'
status: addressed
---
