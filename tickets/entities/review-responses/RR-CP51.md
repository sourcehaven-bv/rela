---
id: RR-CP51
type: review-response
title: 'Cranky #4: arch-lint does not scan _test.go files'
finding: .go-arch-lint.yml excludeFiles skips _test.go, so the mcp->appbuild (and mcp->project) boundary is only enforced for non-test code. A future test stub could re-import internal/project and arch-lint would stay green. Confirmed zero such imports at HEAD; the guarantee is currently held by manual grep, not the linter.
severity: minor
reason: Real but pre-existing and broader than this slice (affects every component boundary, not just mcp). The grep-based lint-test pattern (cf. dataentry/lint_test.go) is the right fix; tracked as a follow-up rather than bolted onto slice 1.
status: deferred
---
