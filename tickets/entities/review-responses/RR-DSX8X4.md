---
id: RR-DSX8X4
type: review-response
title: 'restish test fails in CI: no SPA build, root 404s'
finding: The CI Test job does not build the frontend, so GET / returns 404 and restish ignores Link headers on non-2xx responses. The e2e test would fail in CI and the root-link unit test read the header off a 404.
severity: critical
resolution: Unexported router option withSPAFS lets tests serve a stub SPA (production refuses to start without the build). Both tests use it; the root-link test now also asserts 200. Verified with static/v2 removed locally.
status: addressed
---
