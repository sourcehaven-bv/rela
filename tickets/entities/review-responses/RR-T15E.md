---
id: RR-T15E
type: review-response
title: Middleware fail-loud wraps SPA shell — stamper bug locks operators out of UI
finding: 'attachACLRequest is the outermost wrapper of NewRouter''s chain (router.go:103-105). It wraps mux, which contains /api/, /static/, AND the SPA catch-all at /. With ACL configured and a flapping principal stamper, every request — including GET / (SPA bootstrap) and GET /assets/*.js — 500s with a JSON body, rendering raw JSON in place of the SPA with no UI to recover from. Fix: scope the ACL fail-loud middleware to /api/ only. Either move attachACLRequest INSIDE mux.Handle(''/api/'', ...) so SPA + static assets are exempt, OR keep it at the outer chain but skip non-/api/ paths in the handler body. Pin: ''fail-loud applies to ACL''d endpoints only; SPA shell and static assets must remain reachable.'' Add a regression test: misconfigured stamper + GET / returns 200 with the SPA HTML, not 500 JSON.'
severity: critical
resolution: attachACLRequest scope-checks r.URL.Path against /api/ (and bare /api per RR-P2M7) at the top; non-/api requests pass through unmodified. SPA shell and static assets bypass ACL entirely. TestACLMiddleware_NonAPIPathsBypass pins this for /, /index.html, /static/app.js, /assets/main.css. Wrap-order fix (CRIT-1 / TestACLMiddleware_RouterChainOrder) ensures the API path itself doesn't fail-loud under correct configuration either.
status: addressed
---
