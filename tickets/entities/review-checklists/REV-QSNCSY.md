---
id: REV-QSNCSY
type: review-checklist
title: 'Review: ICS feed serves visible:-redacted properties verbatim'
status: done
---

<!-- @managed: claude-workflow v1 -->

Reconstructed after the fact. BUG-E9DYW5 shipped in PR #1314 as `status=done`
without a review checklist; the malformed frontmatter in
`AM-feed-field-redaction.md` (same PR) aborted entity parsing, so
`done-bug-needs-review-done` never evaluated and CI stayed green. Fixing the
YAML in TKT-IUK0B9 surfaced the gap. Items below record what PR #1314 actually
did — no new verification is claimed.

## Automated Checks
- [x] `go test ./internal/dataentry/` — PASS at merge time (one pre-existing
unrelated failure, `TestBuiltCSSIsLayered`, from stale frontend build artifacts)
- [x] `golangci-lint run ./...` — 0 issues
- [x] Full CI green on PR #1314 before merge

## Code Review
- [x] Reviewed as part of PR #1314 (merged 2026-08-14)
- [x] Fix applied at the single mapping chokepoint in each feed path, mirroring
`affordanceService.copyVisibleProperties`

## Acceptance Verification
- [x] Hidden property never reaches a rendered event
(`TestDeclarativeFeed_RedactsHiddenProperties`)
- [x] Filter-before-redact ORDER pinned, so feed membership cannot vary per
principal (`TestDeclarativeFeed_RedactionDoesNotChangeMembership`)
- [x] Shared store entity not mutated (`TestDeclarativeFeed_RedactionCopies`)
- [x] All three verified to FAIL against the fix being removed, including an
ordering sabotage

## Documentation
- [x] `docs/data-entry.md` and `docs-project/entities/guides/GUIDE-data-entry.md`
updated with the redaction behaviour

## Final Checks
- [x] 5-whys analysis recorded on the bug (why1-why5) with `prevention`
- [x] Durable fix (a distinct type for 'redacted, safe to render') noted on the
bug as tracked separately
- [x] Ready to merge

## Pull Request
- [x] PR #1314 merged to develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1314
