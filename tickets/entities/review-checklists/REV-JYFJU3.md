---
id: REV-JYFJU3
type: review-checklist
title: 'Review: Remote MCP part 1 — go-sdk v1.7.0 migration and ACL-gated read seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

Reconstructed after the fact. TKT-UIR41P shipped as `status=done` without a
review checklist. A malformed-frontmatter parse error made `rela validate` skip
the affected entities and report success, so the workflow gates never failed CI
(see TKT-W76LRP). Items below record what PR #1338 actually did — no new
verification is claimed, except the documentation noted below.

## Automated Checks
- [x] Full CI green on PR #1338 before merge
- [x] Committed MCP goldens pin the tool schemas and error strings byte-for-byte
across the transport swap
- [x] `arch-lint` clean — `internal/mcp` does not import `internal/appbuild`

## Code Review
- [x] Reviewed as part of PR #1338 (merged 2026-08-16)
- [x] Six review responses raised and all resolved to `addressed`: RR-B7ZHYO,
RR-CFFL52, RR-FTJUUE, RR-H7DFZ5, RR-NSUN49, RR-OMB6ID
- [x] Argument-access shim deliberately scoped: porting 26 tools to typed In
structs was deferred as a separately reviewable step, with the rationale
recorded at the call site

## Acceptance Verification
- [x] Migration to go-sdk v1.7.0 is behaviour-preserving — goldens unchanged
- [x] MCP reads routed through the narrow `GraphReader` seam, satisfied by a
visibility-wrapped reader at the wiring site
- [x] Principal required at construction; `NewServer` errors rather than
degrading to an unauthenticated read

## Documentation
- [x] User-facing docs were MISSING at merge and have been written since — see
DOCS-BAE22R and the "Read gating (ACL)" section of `GUIDE-mcp-server.md`

## Final Checks
- [x] Follow-up work tracked as TKT-BDG8U9 (Remote MCP part 2, still backlog)
- [x] Ready to merge

## Pull Request
- [x] PR #1338 merged to develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1338
