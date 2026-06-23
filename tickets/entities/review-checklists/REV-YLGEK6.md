---
id: REV-YLGEK6
type: review-checklist
title: 'Review: Top-of-stack smoke tests: MCP dispatch, router walk, ServeHTTP test convention'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test -race` green on internal/mcp and internal/dataentry
- [x] `golangci-lint run` — 0 issues on both packages
- [x] Full `just ci` green before PR

## Code Review

- [x] `/code-review` run (cranky-code-reviewer on the develop...HEAD diff)
- [x] Findings recorded as review-responses: RR-IP4ZIK (significant, addressed), RR-WJ9GF5 (significant, addressed), RR-TL17B4 (minor, addressed), RR-73G35O (nit, wont-fix with reason), RR-7UMSIM (nit, wont-fix with reason)
- [x] All critical/significant findings addressed (0 critical; both significant fixed: oracle-constraint comments + git probes de-pinned)

## Verification

- [x] Negative property manually verified: removing a route registration fails the walk test with a precise message; tool inventory diff fails by name on unlisted tools
- [x] Three latent fixture bugs surfaced and fixed (nil templater panic in mcp, nil fieldResolver and nil OpenAPIGen panics in dataentry) — documented in IMPL-28A6B0 evidence

**PR:** https://github.com/sourcehaven-bv/rela/pull/956
