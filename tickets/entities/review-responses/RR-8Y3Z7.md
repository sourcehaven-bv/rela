---
id: RR-8Y3Z7
type: review-response
title: frontendparity test-only package is an unusual shape
finding: internal/frontendparity/parity_test.go lives in its own package purely for the parity check. An alternative is `package frontendroutes_test` inside internal/frontendroutes/parity_test.go — compiles as an external test so the leaf-package invariant is preserved. Cosmetic; current shape works.
severity: nit
reason: Cosmetic preference; current test-only package works and makes the cross-boundary intent explicit at the directory level. Can be refactored to package frontendroutes_test later without API impact.
status: deferred
---
