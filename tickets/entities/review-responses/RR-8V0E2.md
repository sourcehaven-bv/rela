---
id: RR-8V0E2
type: review-response
title: Parity regex can silently miss non-literal path expressions
finding: 'internal/frontendparity/parity_test.go:26 regex requires path: ''...'' and name: ''...'' as literal single-quoted strings. A future edit like { path: PATHS.FORM_EDIT, name: ''form-edit'' } wouldn''t match the path capture but name would — silent drift. Fix: assert extracted TS route count equals len(frontendroutes.All()).'
severity: minor
reason: Parity test regex is narrow by design. A future edit that introduces a non-literal path (PATHS.FORM_EDIT etc.) would be a deliberate refactor and the parity test should evolve with it at that point. Adding a route-count check today adds complexity to guard against a refactor that hasn't been requested. Revisit when the TS router grows beyond literal objects.
status: deferred
---
